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

// A byte-level fixture catches delimiter, line-ending, BOM and scalar format
// changes that a parser round-trip alone would not detect.
func TestCanonicalCSVGoldenBytes(t *testing.T) {
	data := CSVInventoryRows{
		Columns: []CSVColumn{"name", "description", "quantity", "purchasePrice", "soldDate", "createdAt", "empty"},
		Rows: [][]any{
			{"雪,é", "quoted \"text\"\nnext", 1.25, -12.5, "2026-05-01", time.Date(2026, 5, 1, 2, 3, 4, 0, time.FixedZone("offset", 3600)), nil},
			{"\t=SUM(A1)", "", 0.00001, 1000.0, "", nil, ""},
		},
	}
	want := "name,description,quantity,purchasePrice,soldDate,createdAt,empty\n" +
		"\"雪,é\",\"quoted \"\"text\"\"\nnext\",1.25,-12.5,2026-05-01,2026-05-01T01:03:04Z,\n" +
		"'\t=SUM(A1),,0.00001,1000,,,\n"
	var first, second bytes.Buffer
	require.NoError(t, WriteCanonicalCSV(&first, data))
	require.NoError(t, WriteCanonicalCSV(&second, data))
	require.Equal(t, []byte(want), first.Bytes())
	require.Equal(t, first.Bytes(), second.Bytes())
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
