package csvexports

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestWriterTypedSafeRoundTrip(t *testing.T) {
	req := Request{Source: "entities", Scope: "collection", Boundary: Boundary{Kind: "all"}, Columns: []Column{{ID: "description", Label: "=heading"}, {ID: "purchasePrice"}, {ID: "name"}, {ID: "createdAt"}}}
	rows := []Row{{"description": "雪, \"quoted\"\nsecond line", "purchasePrice": -42.5, "name": " \t=HYPERLINK(1)", "createdAt": time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}, {"description": "", "purchasePrice": json.Number("-123456789012345"), "name": "-42", "createdAt": nil}}
	var b bytes.Buffer
	if err := Write(&b, req, rows); err != nil {
		t.Fatal(err)
	}
	got, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if got[0][0] != "'=heading" || got[1][0] != rows[0]["description"] || got[1][1] != "-42.5" || got[1][2] != "' \t=HYPERLINK(1)" || got[1][3] != "2026-10-09T00:00:00Z" || got[2][1] != "-123456789012345" || got[2][2] != "'-42" || got[2][3] != "" {
		t.Fatalf("incorrect CSV: %#v", got)
	}
	b.Reset()
	if err := Write(&b, req, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Count(b.String(), "\n") != 1 {
		t.Fatal("empty export missing headers")
	}
	for _, value := range []string{"=1", "+1", "-1", "@x", "\tfoo", "\r=1", "\ufeff=1", "  =1"} {
		got, err := cell(value)
		if err != nil || got != "'"+value {
			t.Fatalf("unsafe %q: %q %v", value, got, err)
		}
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disk failed") }
func TestWriterRejectsHiddenColumnsAndReportsFailure(t *testing.T) {
	req := Request{Source: "api-keys", Scope: "private", Boundary: Boundary{Kind: "all"}, Columns: []Column{{ID: "name"}}}
	if err := Write(failWriter{}, req, []Row{{"name": "safe", "token": "secret"}}); err == nil {
		t.Fatal("lost IO failure")
	}
	req.Columns = []Column{{ID: "token"}}
	if err := Write(io.Discard, req, nil); err == nil {
		t.Fatal("credential exported")
	}
	req.Source = "invitation-visible"
	req.Boundary.Kind = "snapshot"
	if err := Write(io.Discard, req, nil); err == nil {
		t.Fatal("invitation credential exported")
	}
}
