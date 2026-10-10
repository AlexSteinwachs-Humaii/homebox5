package services

import (
	"bytes"
	"encoding/csv"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCanonicalCSV(t *testing.T) {
	values := []any{"雪,\"é\"\nnext", "", nil, " =SUM(A1)", "\t\r\x00-1", "\u200b@cmd", "\u00a0+1", -12.5, 0.00001, true, time.Date(2026, 5, 1, 2, 3, 4, 5, time.FixedZone("x", 3600))}
	cols := make([]CSVColumn, len(values))
	for i := range cols {
		cols[i] = CSVColumn("column" + strings.Repeat("x", i))
	}
	var b bytes.Buffer
	require.NoError(t, WriteCanonicalCSV(&b, CSVInventoryRows{Columns: cols, Rows: [][]any{values}}))
	require.NotContains(t, b.String(), "\ufeff")
	rows, err := csv.NewReader(&b).ReadAll()
	require.NoError(t, err)
	require.Equal(t, []string{"雪,\"é\"\nnext", "", "", "' =SUM(A1)", "'\t\r\x00-1", "'\u200b@cmd", "'\u00a0+1", "-12.5", "0.00001", "true", "2026-05-01T01:03:04.000000005Z"}, rows[1])
}

type csvBadWriter struct{}

func (csvBadWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }
func TestCanonicalCSVRejectsInvalidValuesAndWriteErrors(t *testing.T) {
	for _, v := range []any{math.Inf(1), math.NaN(), []string{"unexpected"}, string([]byte{0xff})} {
		require.Error(t, WriteCanonicalCSV(&bytes.Buffer{}, CSVInventoryRows{Columns: []CSVColumn{"name"}, Rows: [][]any{{v}}}))
	}
	require.ErrorContains(t, WriteCanonicalCSV(csvBadWriter{}, CSVInventoryRows{Columns: []CSVColumn{"name"}}), "disk full")
	require.ErrorContains(t, WriteCanonicalCSV(&bytes.Buffer{}, CSVInventoryRows{Columns: []CSVColumn{"name"}, Rows: [][]any{{}}}), "width mismatch")
}
