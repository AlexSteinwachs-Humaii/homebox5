package repo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
)

func TestCsvExportContractsPersistence(t *testing.T) {
	ctx := context.Background()
	uid := tUser.ID
	gid := tGroup.ID
	r := tRepos.CsvExports
	req := csvexports.Request{Source: "entities", Scope: "collection", Boundary: csvexports.Boundary{Kind: "all"}, Columns: []csvexports.Column{{ID: "name", Label: "Name"}}}
	days, err := r.Retention(ctx, uid, &gid)
	if err != nil || days != 30 {
		t.Fatalf("default collection retention: %d %v", days, err)
	}
	days, err = r.Retention(ctx, uid, nil)
	if err != nil || days != 30 {
		t.Fatalf("default private retention: %d %v", days, err)
	}
	shared, err := r.Create(ctx, uid, &gid, req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tClient.CsvExport.DeleteOneID(shared.ID).Exec(ctx) }()
	if shared.RetentionDays != 30 || shared.AccessRequirement != "member" || shared.Status != csvexport.StatusPending {
		t.Fatal("incorrect captured metadata")
	}
	if err = r.SetRetention(ctx, uid, &gid, csvexports.RetentionSettings{Days: 10}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.SetRetention(ctx, uid, &gid, csvexports.RetentionSettings{Days: 30}) }()
	persisted, err := tClient.CsvExport.Get(ctx, shared.ID)
	if err != nil || persisted.RetentionDays != 30 {
		t.Fatal("policy retroactively changed artifact")
	}
	if err = r.SetRetention(ctx, uid, nil, csvexports.RetentionSettings{Days: 0}); err == nil {
		t.Fatal("accepted zero retention")
	}
	req.Source = "api-keys"
	req.Scope = "private"
	private, err := r.Create(ctx, uid, nil, req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tClient.CsvExport.DeleteOneID(private.ID).Exec(ctx) }()
	if private.GroupID != nil {
		t.Fatal("private row has a tenant")
	}
	if _, err = r.Create(ctx, uid, &gid, req); err == nil {
		t.Fatal("attached private export to collection")
	}
	rows, err := r.CollectionQuery(gid).All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == private.ID {
			t.Fatal("private metadata in shared history")
		}
	}
	rows, err = r.PrivateQuery(uuid.New()).All(ctx)
	if err != nil || len(rows) != 0 {
		t.Fatal("private query not owner-scoped")
	}
	// The DB guards against accidentally dropping the only artifact pointer.
	if err = tClient.User.DeleteOneID(uid).Exec(ctx); err == nil {
		t.Fatal("requester deleted before CSV cleanup")
	}
	if err = tClient.Group.DeleteOneID(gid).Exec(ctx); err == nil {
		t.Fatal("collection deleted before CSV cleanup")
	}
	if err = tClient.CsvExport.UpdateOneID(private.ID).SetStatus(csvexport.StatusCompleted).Exec(ctx); err == nil {
		t.Fatal("completed row without artifact/expiry")
	}
	if _, err = tClient.CsvExport.Create().SetRequesterID(uid).SetGroupID(gid).SetScope(csvexport.ScopePrivate).SetSource(req.Source).SetAccessRequirement("requester").SetRequestSnapshot(req).Save(ctx); err == nil {
		t.Fatal("DB permits mixed scope")
	}
}

func TestCsvPrivateExportWithoutCollections(t *testing.T) {
	ctx := context.Background()
	u, err := tClient.User.Create().SetName("CSV private account").SetEmail(uuid.NewString() + "@csv.test").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tClient.User.DeleteOneID(u.ID).Exec(ctx) }()
	if count, err := u.QueryGroups().Count(ctx); err != nil || count != 0 {
		t.Fatal("expected account without memberships")
	}
	req := csvexports.Request{Source: "api-keys", Scope: "private", Boundary: csvexports.Boundary{Kind: "all"}, Columns: []csvexports.Column{{ID: "name"}}}
	row, err := tRepos.CsvExports.Create(ctx, u.ID, nil, req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tClient.CsvExport.DeleteOneID(row.ID).Exec(ctx) }()
	if row.RetentionDays != 30 || row.GroupID != nil {
		t.Fatal("private policy depends on tenant")
	}
	if err = tRepos.CsvExports.SetRetention(ctx, u.ID, nil, csvexports.RetentionSettings{Days: 5}); err != nil {
		t.Fatal(err)
	}
	days, err := tRepos.CsvExports.Retention(ctx, u.ID, nil)
	if err != nil || days != 5 {
		t.Fatal("private setting not saved")
	}
	stored, err := tClient.CsvExport.UpdateOneID(row.ID).SetArtifactPath("private/storage/secret.csv").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(CsvExportMetadata(stored))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "storage") || strings.Contains(string(data), "artifactPath") {
		t.Fatal("public metadata disclosed artifact key")
	}
	if stored.RetentionDays != 30 {
		t.Fatal("private policy applied retroactively")
	}
}
