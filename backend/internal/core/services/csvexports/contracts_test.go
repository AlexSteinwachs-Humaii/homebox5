package csvexports

import (
	"encoding/json"
	"strings"
	"testing"
)

const validRequest = `{"source":"entities","scope":"collection","filters":{"search":"café"},"boundary":{"kind":"all"},"sort":[{"column":"name","direction":"asc"}],"columns":[{"id":"name","label":"Name"},{"id":"assetId","label":"Asset ID"}]}`

func TestDecodeCSVRequest(t *testing.T) {
	req, err := DecodeRequest(strings.NewReader(validRequest))
	if err != nil {
		t.Fatal(err)
	}
	if req.Columns[0].ID != "name" || req.Columns[1].ID != "assetId" {
		t.Fatal("lost visible order")
	}
	tests := map[string]string{
		"SQL":                strings.Replace(validRequest, `"filters":{`, `"filters":{"sql":"SELECT *",`, 1),
		"blob":               strings.Replace(validRequest, `"filters":{`, `"filters":{"blobKey":"secret",`, 1),
		"user":               strings.Replace(validRequest, `"source":`, `"requesterId":"admin","source":`, 1),
		"permission":         strings.Replace(validRequest, `"source":`, `"isOwner":true,"source":`, 1),
		"hidden":             strings.Replace(validRequest, `"id":"name"`, `"id":"password"`, 1),
		"actions":            strings.Replace(validRequest, `"id":"name"`, `"id":"actions"`, 1),
		"wrong scope":        strings.Replace(validRequest, `"collection"`, `"private"`, 1),
		"unknown source":     strings.Replace(validRequest, `"entities"`, `"database"`, 1),
		"duplicate column":   strings.Replace(validRequest, `"id":"assetId"`, `"id":"name"`, 1),
		"sort":               strings.Replace(validRequest, `"asc"`, `"SQL"`, 1),
		"unsupported filter": strings.Replace(validRequest, `"search":"café"`, `"status":"completed"`, 1),
		"page":               strings.Replace(validRequest, `"search":"café"`, `"page":1`, 1),
		"limit":              strings.Replace(validRequest, `"kind":"all"`, `"kind":"all","limit":5`, 1),
		"second document":    validRequest + ` {}`,
		"retention":          strings.Replace(validRequest, `"source":`, `"retentionDays":3,"source":`, 1),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeRequest(strings.NewReader(data)); err == nil {
				t.Fatal("accepted unsafe request")
			}
		})
	}
}
func TestRegistryContracts(t *testing.T) {
	for id, s := range registry {
		req := Request{Source: id, Scope: s.Scope, Boundary: Boundary{Kind: s.Boundaries[0]}, Columns: []Column{{ID: s.Columns[0], Label: "Visible"}}}
		switch req.Boundary.Kind {
		case "children":
			var filters Filters
			_ = json.Unmarshal([]byte(`{"entityId":"00000000-0000-0000-0000-000000000001"}`), &filters)
			if id == "template-fields" {
				_ = json.Unmarshal([]byte(`{"templateId":"00000000-0000-0000-0000-000000000001"}`), &filters)
				filters.EntityID = nil
			}
			req.Filters = filters
		}
		if err := req.Validate(); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		req.Columns[0].ID = "artifact_path"
		if req.Validate() == nil {
			t.Fatalf("%s permits blob keys", id)
		}
	}
	s, _ := Lookup("entities")
	s.Columns[0] = "password"
	fresh, _ := Lookup("entities")
	if fresh.Columns[0] == "password" {
		t.Fatal("mutable registry")
	}
}
func TestRetentionContracts(t *testing.T) {
	for _, days := range []int{1, 30, MaxRetentionDays} {
		if err := (RetentionSettings{Days: days}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, days := range []int{-1, 0, MaxRetentionDays + 1} {
		if (RetentionSettings{Days: days}).Validate() == nil {
			t.Fatal("accepted invalid retention")
		}
	}
	var settings RetentionSettings
	if err := json.Unmarshal([]byte(`{"days":1.5}`), &settings); err == nil {
		t.Fatal("accepted fractional days")
	}
}
func TestBoundariesAndSnapshots(t *testing.T) {
	base, _ := DecodeRequest(strings.NewReader(validRequest))
	base.Boundary = Boundary{Kind: "recent"}
	if base.Validate() == nil {
		t.Fatal("unbounded recent set")
	}
	base.Boundary.Limit = 5
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	base = Request{Source: "draft-fields", Scope: "private", Boundary: Boundary{Kind: "snapshot"}, Columns: []Column{{ID: "name"}}, Snapshot: []SnapshotRow{{Name: "Draft", Token: "secret"}}}
	if base.Validate() == nil {
		t.Fatal("snapshot allows hidden token")
	}
	base.Snapshot[0].Token = ""
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	base.Scope = "collection"
	if base.Validate() == nil {
		t.Fatal("draft leaked to shared history")
	}
}
