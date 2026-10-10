package csvexports

import (
	"fmt"
	"github.com/google/uuid"
	"testing"
)

func TestResolveCompleteOrderBoundariesAndProjection(t *testing.T) {
	rows := []Row{}
	ids := []uuid.UUID{}
	for i := 0; i < 257; i++ {
		id := uuid.New()
		ids = append(ids, id)
		rows = append(rows, Row{"id": id.String(), "name": fmt.Sprintf("Record %d", i), "quantity": i, "passwordHash": "secret"})
	}
	req := Request{Source: "entities", Scope: "collection", Boundary: Boundary{Kind: "all"}, Columns: []Column{{ID: "name"}, {ID: "quantity"}}, Sort: []Sort{{Column: "quantity", Direction: "desc"}}}
	got, err := Resolve(req, rows)
	if err != nil || len(got) != 257 || got[0]["quantity"] != 256 {
		t.Fatalf("full numeric sort: %d %v", len(got), err)
	}
	if _, ok := got[0]["passwordHash"]; ok {
		t.Fatal("hidden data survived projection")
	}
	req.Sort = nil
	req.Boundary = Boundary{Kind: "explicit", RecordIDs: []uuid.UUID{ids[200], ids[1], ids[99]}, ExcludeIDs: []uuid.UUID{ids[1]}}
	got, err = Resolve(req, rows)
	if err != nil || len(got) != 2 || got[0]["quantity"] != 200 || got[1]["quantity"] != 99 {
		t.Fatal("explicit order/exclusions lost", got, err)
	}
	req.Boundary.RecordIDs = nil
	got, err = Resolve(req, rows)
	if err != nil || len(got) != 0 {
		t.Fatal("empty explicit became all")
	}
	req.Boundary = Boundary{Kind: "recent", Limit: 5}
	got, err = Resolve(req, rows)
	if err != nil || len(got) != 5 {
		t.Fatal("recent limit lost")
	}
	req.Source = "locations"
	req.Boundary = Boundary{Kind: "roots"}
	req.Columns = []Column{{ID: "name"}}
	got, err = Resolve(req, []Row{{"id": ids[0].String(), "name": "root"}, {"id": ids[1].String(), "name": "child", "parentId": ids[0].String()}})
	if err != nil || len(got) != 1 {
		t.Fatal("root boundary lost")
	}
}
