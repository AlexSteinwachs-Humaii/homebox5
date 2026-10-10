package csvexports

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

func number(v any) (*big.Rat, bool) {
	var text string
	switch n := v.(type) {
	case int:
		text = strconv.Itoa(n)
	case int64:
		text = strconv.FormatInt(n, 10)
	case float64:
		text = strconv.FormatFloat(n, 'g', -1, 64)
	case json.Number:
		text = string(n)
	default:
		return nil, false
	}
	return new(big.Rat).SetString(text)
}

func cell(v any) (string, error) {
	if v == nil {
		return "", nil
	}
	switch x := v.(type) {
	case string:
		// Leading whitespace/control characters must not hide a spreadsheet formula.
		probe := strings.TrimLeft(x, " \t\r\n\v\f\ufeff")
		if strings.ContainsAny(x[:len(x)-len(probe)], "\t\r\n") || len(probe) > 0 && strings.ContainsRune("=+-@", rune(probe[0])) {
			return "'" + x, nil
		}
		return x, nil
	case time.Time:
		if x.IsZero() {
			return "", nil
		}
		return x.Format(time.RFC3339Nano), nil
	case json.Number:
		n, err := strconv.ParseFloat(string(x), 64)
		if err != nil {
			return "", err
		}
		if math.IsInf(n, 0) || math.IsNaN(n) {
			return "", fmt.Errorf("non-finite CSV number")
		}
		return string(x), nil
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return "", fmt.Errorf("non-finite CSV number")
		}
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case int, int64, bool:
		return fmt.Sprint(x), nil
	default:
		return "", fmt.Errorf("unsupported CSV cell type %T", v)
	}
}

// Write emits headers even for an empty source. No DOM text or implicit columns.
// Labels are also untrusted text and receive formula protection.
func Write(w io.Writer, req Request, rows []Row) error {
	if err := req.Validate(); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	header := make([]string, len(req.Columns))
	for i, c := range req.Columns {
		label := c.Label
		if label == "" {
			label = c.ID
		}
		var err error
		header[i], err = cell(label)
		if err != nil {
			return err
		}
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, len(req.Columns))
		for i, c := range req.Columns {
			var err error
			record[i], err = cell(row[c.ID])
			if err != nil {
				return fmt.Errorf("column %s: %w", c.ID, err)
			}
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
