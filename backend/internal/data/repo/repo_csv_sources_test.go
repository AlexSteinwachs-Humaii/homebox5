package repo

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
)

func TestCsvSourcesEntireEntitySetAndIsolation(t *testing.T) {
	ctx := context.Background()
	g, err := tRepos.Groups.GroupCreate(ctx, "csv source tests", tUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer tRepos.Groups.GroupDelete(ctx, g.ID)
	ids := []uuid.UUID{}
	for i := 0; i < 105; i++ {
		e, err := tRepos.Entities.Create(ctx, g.ID, EntityCreate{Name: fmt.Sprintf("CSV %03d", i), Quantity: float64(i)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, e.ID)
	}
	req := csvexports.Request{Source: "entities", Scope: "collection", Boundary: csvexports.Boundary{Kind: "all"}, Columns: []csvexports.Column{{ID: "name"}, {ID: "quantity"}}, Sort: []csvexports.Sort{{Column: "quantity", Direction: "desc"}}}
	rows, err := tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 105 || fmt.Sprint(rows[0]["quantity"]) != "104" {
		t.Fatalf("truncated/incorrect numeric order: %d %#v", len(rows), rows[0])
	}
	req.Boundary = csvexports.Boundary{Kind: "explicit", RecordIDs: []uuid.UUID{ids[100], ids[1]}}
	req.Sort = nil
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 2 || rows[0]["id"] != ids[100].String() {
		t.Fatal(rows, err)
	}
	req.Boundary = csvexports.Boundary{Kind: "recent", Limit: 5}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 5 {
		t.Fatal("recent", rows, err)
	}
	req.Filters.Search = "CSV 100"
	req.Boundary = csvexports.Boundary{Kind: "all"}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 1 {
		t.Fatal("search", rows, err)
	}
	if _, err := tRepos.CsvExports.ResolveSource(ctx, uuid.New(), &g.ID, req); err == nil {
		t.Fatal("unrelated requester accepted")
	}
	req.Source = "label-records"
	req.Scope = "private"
	req.Filters = csvexports.Filters{}
	req.Boundary = csvexports.Boundary{Kind: "explicit", RecordIDs: []uuid.UUID{ids[1]}}
	req.Columns = []csvexports.Column{{ID: "name"}}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, nil, req)
	if err != nil || len(rows) != 1 {
		t.Fatal("private reference", rows, err)
	}
	req.Source = "draft-fields"
	req.Boundary = csvexports.Boundary{Kind: "snapshot"}
	req.Snapshot = []csvexports.SnapshotRow{{Name: "=unsafe", Value: "Unicode 雪"}}
	req.Columns = []csvexports.Column{{ID: "name"}, {ID: "value"}}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, nil, req)
	if err != nil || rows[0]["value"] != "Unicode 雪" {
		t.Fatal("snapshot", rows, err)
	}
}
func TestCsvSourcesEveryRegistryAdapter(t *testing.T) {
	ctx := context.Background()
	g, err := tRepos.Groups.GroupCreate(ctx, "csv adapters", tUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer tRepos.Groups.GroupDelete(ctx, g.ID)
	e, err := tRepos.Entities.Create(ctx, g.ID, EntityCreate{Name: "Context"})
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := tRepos.EntityTemplates.Create(ctx, g.ID, EntityTemplateCreate{Name: "Template", Fields: []TemplateField{{Name: "Number", Type: "number", NumberValue: -8}}})
	if err != nil {
		t.Fatal(err)
	}
	sources := []string{"entities", "asset-results", "locations", "tags", "templates", "template-fields", "maintenance", "members", "invitations", "notifiers", "entity-types", "attachments", "entity-fields", "entity-tags", "entity-path", "zip-history", "collection-csv-history", "private-csv-history", "api-keys", "collections", "draft-fields", "draft-photos", "scanner-results", "label-records", "invitation-visible", "product-results"}
	for _, id := range sources {
		t.Run(id, func(t *testing.T) {
			s, _ := csvexports.Lookup(id)
			req := csvexports.Request{Source: id, Scope: s.Scope, Boundary: csvexports.Boundary{Kind: s.Boundaries[0]}, Columns: []csvexports.Column{{ID: s.Columns[0]}}}
			if req.Boundary.Kind == "children" {
				if id == "template-fields" {
					req.Filters.TemplateID = &tmpl.ID
				} else {
					req.Filters.EntityID = &e.ID
				}
			}
			if id == "entity-path" {
				req.Filters.EntityID = &e.ID
			}
			if id == "asset-results" {
				a := 0
				req.Filters.AssetID = &a
			}
			if id == "maintenance" {
				req.Filters.Status = "both"
			}
			if _, err := tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCsvSourcesHierarchyChildrenSecretsAndNoCollection(t *testing.T) {
	ctx := context.Background()
	u, err := tRepos.Users.Create(ctx, userFactory())
	if err != nil {
		t.Fatal(err)
	}
	key, err := tRepos.APIKeys.Create(ctx, u.ID, "Key 名", []byte("do-not-export-this-token-hash"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tRepos.APIKeys.Delete(ctx, u.ID, key.ID)
	req := csvexports.Request{Source: "api-keys", Scope: "private", Boundary: csvexports.Boundary{Kind: "all"}, Columns: []csvexports.Column{{ID: "name"}, {ID: "expiresAt"}}}
	rows, err := tRepos.CsvExports.ResolveSource(ctx, u.ID, nil, req)
	if err != nil || len(rows) != 1 || rows[0]["name"] != "Key 名" {
		t.Fatal("account-only metadata", rows, err)
	}
	if _, ok := rows[0]["token"]; ok {
		t.Fatal("key secret exposed")
	}
	if _, ok := rows[0]["userId"]; ok {
		t.Fatal("hidden field exposed")
	}
	g, err := tRepos.Groups.GroupCreate(ctx, "hierarchy", tUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer tRepos.Groups.GroupDelete(ctx, g.ID)
	root, err := tRepos.Entities.CreateContainer(ctx, g.ID, EntityCreate{Name: "Root"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := tRepos.Entities.Create(ctx, g.ID, EntityCreate{Name: "Child", ParentID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	grandchild, err := tRepos.Entities.Create(ctx, g.ID, EntityCreate{Name: "Nested", ParentID: child.ID})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	req = csvexports.Request{Source: "locations", Scope: "collection", Filters: csvexports.Filters{ShowItems: &yes}, Boundary: csvexports.Boundary{Kind: "hierarchy"}, Columns: []csvexports.Column{{ID: "name"}, {ID: "path"}}}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 3 || rows[2]["path"] != "Root / Child / Nested" {
		t.Fatal("collapsed hierarchy omitted records/context", rows, err)
	}
	req.Boundary.Kind = "all"
	req.Columns = append(req.Columns, csvexports.Column{ID: "itemCount"})
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 2 || fmt.Sprint(rows[0]["itemCount"]) != "0" {
		t.Fatal("location source within transaction", rows, err)
	}
	if _, err := tRepos.CsvExports.ResolveSource(ctx, u.ID, &g.ID, req); err == nil {
		t.Fatal("nonmember exported collection")
	}
	req = csvexports.Request{Source: "entities", Scope: "collection", Filters: csvexports.Filters{EntityID: &child.ID}, Boundary: csvexports.Boundary{Kind: "children"}, Columns: []csvexports.Column{{ID: "name"}}}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 1 || rows[0]["id"] != grandchild.ID.String() {
		t.Fatal("child boundary", rows, err)
	}
	req.Filters.ParentIDs = []uuid.UUID{root.ID}
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 0 {
		t.Fatal("conflicting parent predicate ignored")
	}
	req = csvexports.Request{Source: "scanner-results", Scope: "private", Boundary: csvexports.Boundary{Kind: "explicit", RecordIDs: []uuid.UUID{child.ID}}, Columns: []csvexports.Column{{ID: "name"}}}
	if _, err := tRepos.CsvExports.ResolveSource(ctx, u.ID, nil, req); err == nil {
		t.Fatal("snapshot granted entity permission")
	}
}
