package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func csvTestCapture(s CSVSourceSpec) CSVCaptureRequest {
	r := CSVCaptureRequest{Source: s.Source, Scope: s.Scope, Columns: []CSVColumn{s.Columns[0]}, Ordering: []CSVOrder{{Column: "id", Direction: "asc"}}}
	if s.Scope == CSVCollection {
		id := uuid.New()
		r.CollectionID = &id
	}
	if s.ContextKind != "" {
		r.Hierarchy = &CSVHierarchyContext{Kind: s.ContextKind, RootID: uuid.New(), Descendants: s.Adapter != CSVAttachments && s.Source != "entity-maintenance"}
	}
	if s.SelectorKind != "" {
		r.Selector = &CSVSelectorCapture{Kind: s.SelectorKind, Eligibility: "explicit-set", EligibleIDs: []uuid.UUID{uuid.New()}}
	}
	if s.Recent {
		r.Recent = &CSVRecentBoundary{CapturedAt: time.Now().UTC(), RecordIDs: []uuid.UUID{uuid.New()}}
	}
	return r
}

func TestCSVSourceCoverage(t *testing.T) {
	expected := strings.Fields("items locations dashboard-recent-items entity-inventory tag-inventory entity-hierarchy item-selector location-selector entity-selector tags tag-hierarchy tag-selector entity-types entity-type-selector templates template-selector maintenance entity-maintenance collection-members collection-invitations collection-notifiers entity-attachments entity-media backup-history collection-csv-history private-csv-history account-api-keys account-collections collection-selector")
	catalogue := CSVSourceCatalogue()
	if len(catalogue) != len(expected) {
		t.Fatalf("coverage count: %d", len(catalogue))
	}
	seen := map[CSVSource]bool{}
	for _, s := range catalogue {
		if seen[s.Source] {
			t.Fatalf("duplicate %s", s.Source)
		}
		seen[s.Source] = true
		if s.Adapter == "" || s.Access == "" || len(s.Columns) == 0 {
			t.Fatalf("incomplete spec: %+v", s)
		}
		r := csvTestCapture(s)
		if err := r.Validate(); err != nil {
			t.Fatalf("%s: %v", s.Source, err)
		}
		b, _ := json.Marshal(r)
		if _, err := DecodeCSVCapture(strings.NewReader(string(b))); err != nil {
			t.Fatalf("%s roundtrip: %v", s.Source, err)
		}
		for _, secret := range strings.Fields("token tokenHash password hash pepper credential url path artifactPath thumbnail actions checkbox theme language currencyChoice") {
			bad := r
			bad.Columns = []CSVColumn{CSVColumn(secret)}
			if bad.Validate() == nil {
				t.Fatalf("%s permits %s", s.Source, secret)
			}
		}
	}
	for _, source := range expected {
		if !seen[CSVSource(source)] {
			t.Fatalf("missing %s", source)
		}
	}
	for _, source := range strings.Fields("navigation theme language currency print-layout") {
		if _, ok := CSVSourceDefinition(CSVSource(source)); ok {
			t.Fatal(source)
		}
	}
	catalogue[0].Columns[0] = "token"
	fresh, _ := CSVSourceDefinition("items")
	if fresh.Columns[0] != "id" {
		t.Fatal("mutable policy")
	}
}

func TestCSVCaptureRejectsTampering(t *testing.T) {
	s, _ := CSVSourceDefinition("items")
	tests := map[string]func(*CSVCaptureRequest){
		"source":         func(r *CSVCaptureRequest) { r.Source = "sql" },
		"scope":          func(r *CSVCaptureRequest) { r.Scope = CSVAccountPrivate },
		"collection":     func(r *CSVCaptureRequest) { r.CollectionID = nil },
		"columns":        func(r *CSVCaptureRequest) { r.Columns = []CSVColumn{"id", "id"} },
		"direction":      func(r *CSVCaptureRequest) { r.Ordering[0].Direction = "DESC; DROP" },
		"sort":           func(r *CSVCaptureRequest) { r.Ordering[0].Column = "token" },
		"duplicate sort": func(r *CSVCaptureRequest) { r.Ordering = append(r.Ordering, r.Ordering[0]) },
		"wrong filter":   func(r *CSVCaptureRequest) { r.Maintenance = &CSVMaintenanceFilters{Status: "both"} },
		"wrong text":     func(r *CSVCaptureRequest) { r.Text = &CSVTextFilters{Search: "x"} },
		"hierarchy":      func(r *CSVCaptureRequest) { r.Hierarchy = &CSVHierarchyContext{RootID: uuid.New(), Kind: "entity"} },
		"recent":         func(r *CSVCaptureRequest) { r.Recent = &CSVRecentBoundary{CapturedAt: time.Now()} },
		"selector":       func(r *CSVCaptureRequest) { r.Selector = &CSVSelectorCapture{Kind: "item"} },
		"photo": func(r *CSVCaptureRequest) {
			r.Entities = &CSVEntityFilters{OnlyWithPhoto: true, OnlyWithoutPhoto: true}
		},
		"nil reference": func(r *CSVCaptureRequest) { r.Entities = &CSVEntityFilters{TagIDs: []uuid.UUID{uuid.Nil}} },
		"duplicate reference": func(r *CSVCaptureRequest) {
			id := uuid.New()
			r.Entities = &CSVEntityFilters{ParentIDs: []uuid.UUID{id, id}}
		},
		"asset zero":        func(r *CSVCaptureRequest) { r.Entities = &CSVEntityFilters{AssetID: "0"} },
		"asset injection":   func(r *CSVCaptureRequest) { r.Entities = &CSVEntityFilters{AssetID: "\"1\""} },
		"wrong entity kind": func(r *CSVCaptureRequest) { v := true; r.Entities = &CSVEntityFilters{IsLocation: &v} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			r := csvTestCapture(s)
			mutate(&r)
			if r.Validate() == nil {
				t.Fatal("accepted tampered capture")
			}
		})
	}
	for _, source := range []CSVSource{"account-api-keys", "private-csv-history", "account-collections", "collection-selector"} {
		s, _ := CSVSourceDefinition(source)
		r := csvTestCapture(s)
		if err := r.Validate(); err != nil {
			t.Fatal(err)
		}
		id := uuid.New()
		r.CollectionID = &id
		if r.Validate() == nil {
			t.Fatal("private source accepted collection")
		}
	}
}

func TestCSVStrictJSON(t *testing.T) {
	s, _ := CSVSourceDefinition("account-api-keys")
	b, _ := json.Marshal(csvTestCapture(s))
	base := string(b)
	for _, extra := range []string{`"page":1`, `"pageSize":5`, `"userId":"x"`, `"role":"owner"`, `"blobPath":"x"`, `"source":"items"`, `"Source":"items"`, `"SOURCE":"account-api-keys"`, `"entities":{"search":"a","search":"b"}`} {
		input := strings.TrimSuffix(base, "}") + "," + extra + "}"
		if _, err := DecodeCSVCapture(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %s", extra)
		}
	}
	for _, input := range []string{"null", base + base, `[]`, strings.Repeat(" ", 1<<20+1), strings.Replace(base, `"ordering":`, `"ordering":[{"column":"id","direction":"asc","page":1}],"unused":`, 1)} {
		if _, err := DecodeCSVCapture(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted malformed JSON")
		}
	}
}

func TestCSVRecentAndSelectorBoundaries(t *testing.T) {
	s, _ := CSVSourceDefinition("dashboard-recent-items")
	r := csvTestCapture(s)
	captured := r.Recent.CapturedAt
	// Serialization after a queue delay preserves membership and timestamp.
	b, _ := json.Marshal(r)
	clone, err := DecodeCSVCapture(strings.NewReader(string(b)))
	if err != nil || !clone.Recent.CapturedAt.Equal(captured) || clone.Recent.RecordIDs[0] != r.Recent.RecordIDs[0] {
		t.Fatalf("lost boundary: %v", err)
	}
	r.Recent.RecordIDs = []uuid.UUID{}
	if r.Validate() != nil {
		t.Fatal("empty recent set rejected")
	}
	r.Recent.RecordIDs = nil
	if r.Validate() == nil {
		t.Fatal("missing recent membership accepted")
	}
	r = csvTestCapture(s)
	r.Recent = nil
	if r.Validate() == nil {
		t.Fatal("missing recent boundary accepted")
	}
	s, _ = CSVSourceDefinition("item-selector")
	r = csvTestCapture(s)
	r.Selector.EligibleIDs = []uuid.UUID{}
	if r.Validate() != nil {
		t.Fatal("empty eligibility rejected")
	}
	r.Selector.ExcludedIDs = []uuid.UUID{uuid.New()}
	if r.Validate() == nil {
		t.Fatal("foreign exclusion accepted")
	}
	r = csvTestCapture(s)
	r.Selector.Eligibility = "all"
	if r.Validate() == nil {
		t.Fatal("widening eligibility accepted")
	}
	s, _ = CSVSourceDefinition("maintenance")
	r = csvTestCapture(s)
	r.Maintenance = &CSVMaintenanceFilters{Status: "unknown"}
	if r.Validate() == nil {
		t.Fatal("unknown maintenance status")
	}
}

type csvTestAuthority struct {
	called bool
	err    error
}

func (a *csvTestAuthority) AuthorizeCSVCapture(_ context.Context, _ uuid.UUID, _ CSVCaptureRequest, _ CSVSourceSpec) error {
	a.called = true
	return a.err
}
func TestCSVCaptureRequiresAuthority(t *testing.T) {
	s, _ := CSVSourceDefinition("account-api-keys")
	r := csvTestCapture(s)
	user := uuid.New()
	if ValidateCSVCapture(context.Background(), user, r, nil) == nil {
		t.Fatal("missing authority accepted")
	}
	a := &csvTestAuthority{err: errors.New("foreign reference")}
	if ValidateCSVCapture(context.Background(), uuid.Nil, r, a) == nil || a.called {
		t.Fatal("anonymous request reached authority")
	}
	r.Columns = []CSVColumn{"token"}
	if ValidateCSVCapture(context.Background(), user, r, a) == nil || a.called {
		t.Fatal("malformed request reached authority")
	}
	r = csvTestCapture(s)
	if err := ValidateCSVCapture(context.Background(), user, r, a); !errors.Is(err, a.err) || !a.called {
		t.Fatal("authority denial ignored")
	}
	a.err = nil
	if err := ValidateCSVCapture(context.Background(), user, r, a); err != nil {
		t.Fatal(err)
	}
}
