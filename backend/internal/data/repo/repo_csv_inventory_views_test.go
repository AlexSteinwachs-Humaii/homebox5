package repo

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
)

func TestCsvInventoryViewRequestsAndBytes(t *testing.T) {
	ctx := context.Background()
	g, err := tRepos.Groups.GroupCreate(ctx, "CSV inventory views", tUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer tRepos.Groups.GroupDelete(ctx, g.ID)
	root, err := tRepos.Entities.CreateContainer(ctx, g.ID, EntityCreate{Name: "Visible root"})
	if err != nil {
		t.Fatal(err)
	}
	archived, err := tRepos.Entities.CreateContainer(ctx, g.ID, EntityCreate{Name: "Archived root"})
	if err != nil {
		t.Fatal(err)
	}
	if err := tRepos.Entities.db.Entity.UpdateOneID(archived.ID).SetArchived(true).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 55; i++ {
		_, err := tRepos.Entities.Create(ctx, g.ID, EntityCreate{Name: fmt.Sprintf("Lamp 灯 %02d", i), Quantity: float64(i + 1), ParentID: root.ID})
		if err != nil {
			t.Fatal(err)
		}
	}
	// This is the same children + ordered table context sent by the embedded view.
	raw := fmt.Sprintf(`{"source":"entities","scope":"collection","filters":{"entityId":%q},"boundary":{"kind":"children"},"sort":[{"column":"quantity","direction":"desc"}],"columns":[{"id":"quantity","label":"Quantity"},{"id":"name","label":"Name"}]}`, root.ID.String())
	req, err := csvexports.DecodeRequest(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := csvexports.Write(&output, req, rows); err != nil {
		t.Fatal(err)
	}
	cells, err := csv.NewReader(&output).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 56 || strings.Join(cells[0], ",") != "Quantity,Name" || cells[1][0] != "55" || cells[55][0] != "1" {
		t.Fatalf("page-limited or incorrect displayed columns/order: %d %v", len(cells), cells)
	}
	// Root cards use the non-archived root set; hierarchies include collapsed items.
	req = csvexports.Request{Source: "locations", Scope: "collection", Boundary: csvexports.Boundary{Kind: "roots"}, Columns: []csvexports.Column{{ID: "name", Label: "Name"}, {ID: "itemCount", Label: "Items"}}}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 1 || rows[0]["id"] != root.ID.String() {
		t.Fatal("root card boundary", rows, err)
	}
	show := true
	req.Boundary.Kind = "hierarchy"
	req.Filters.ShowItems = &show
	req.Columns = []csvexports.Column{{ID: "name"}, {ID: "parentId"}, {ID: "path"}}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 57 {
		t.Fatal("hierarchy must not use a page/collapse slice", len(rows), err)
	}
	show = false
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 2 {
		t.Fatal("show-items boundary", rows, err)
	}
	parentItem, err := tRepos.Entities.Create(ctx, g.ID, EntityCreate{Name: "Parent item", ParentID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tRepos.Entities.Create(ctx, g.ID, EntityCreate{Name: "Nested item", ParentID: parentItem.ID})
	if err != nil {
		t.Fatal(err)
	}
	req = csvexports.Request{Source: "entities", Scope: "collection", Filters: csvexports.Filters{EntityID: &parentItem.ID}, Boundary: csvexports.Boundary{Kind: "children"}, Columns: []csvexports.Column{{ID: "name"}, {ID: "location"}, {ID: "path"}}}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 1 || rows[0]["location"] != "Parent item" || rows[0]["path"] != "Visible root / Parent item / Nested item" {
		t.Fatal("inventory parent field/context differ from displayed summary", rows, err)
	}
}
