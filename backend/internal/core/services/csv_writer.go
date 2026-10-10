package services

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

var errCSVEncoding = errors.New("csv: invalid encoding")

// WriteCanonicalCSV writes UTF-8, comma-delimited RFC4180 records with LF line
// endings, no BOM, and ordered machine column names. There is no row limit.
func WriteCanonicalCSV(dst io.Writer, data CSVInventoryRows) error {
	w := csv.NewWriter(dst)
	header := make([]string, len(data.Columns))
	for i, col := range data.Columns {
		header[i] = string(col)
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, row := range data.Rows {
		if len(row) != len(header) {
			return fmt.Errorf("%w: row width mismatch", errCSVEncoding)
		}
		record := make([]string, len(row))
		for i, value := range row {
			text, err := csvScalar(value)
			if err != nil {
				return err
			}
			record[i] = text
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func csvScalar(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		if !utf8.ValidString(x) {
			return "", fmt.Errorf("%w: invalid UTF-8", errCSVEncoding)
		}
		// Ignore leading Unicode whitespace, format characters and controls when
		// detecting formulas. Preserve original text, prefixing an apostrophe.
		probe := strings.TrimLeftFunc(x, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) })
		if len(probe) > 0 && strings.ContainsRune("=+-@", rune(probe[0])) {
			return "'" + x, nil
		}
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return "", fmt.Errorf("%w: non-finite number", errCSVEncoding)
		}
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano), nil
	case uuid.UUID:
		return x.String(), nil
	default:
		return "", fmt.Errorf("%w: unsupported scalar %T", errCSVEncoding, v)
	}
}
