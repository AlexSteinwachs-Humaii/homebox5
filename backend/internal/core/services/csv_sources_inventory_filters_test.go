package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/attachment"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entityfield"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/data/types"
)

func TestCSVInventoryMatchesEntityPredicates(t *testing.T) {
	ctx := context.Background()
	gid, loc, box, tg := csvInventoryFixture(t)
	all, err := tRepos.Entities.GetAll(ctx, gid)
	require.NoError(t, err)
	var leaf repo.EntityOut
	for _, e := range all {
		if e.Parent != nil && e.Parent.ID == box {
			leaf = e
			break
		}
	}
	require.NoError(t, tClient.Entity.UpdateOneID(leaf.ID).SetSerialNumber("serial-match").SetArchived(true).Exec(ctx))
	_, err = tClient.EntityField.Create().SetEntityID(leaf.ID).SetType(entityfield.TypeText).SetName("Custom").SetTextValue("value").Save(ctx)
	require.NoError(t, err)
	_, err = tClient.Attachment.Create().SetEntityID(box).SetTitle("photo").SetPath("not-exported").SetType(attachment.TypePhoto).SetPrimary(true).Save(ctx)
	require.NoError(t, err)
	s := NewCSVInventorySources(tClient, nil)
	tests := []struct {
		name   string
		filter CSVEntityFilters
		query  repo.EntityQuery
	}{
		{"serial search", CSVEntityFilters{Search: "serial-match", IncludeArchived: true}, repo.EntityQuery{Search: "serial-match", IncludeArchived: true}},
		{"archived omitted", CSVEntityFilters{Search: "serial-match"}, repo.EntityQuery{Search: "serial-match"}},
		{"parent", CSVEntityFilters{ParentIDs: []uuid.UUID{loc}}, repo.EntityQuery{ParentIDs: []uuid.UUID{loc}}},
		{"parent item", CSVEntityFilters{ParentItemIDs: []uuid.UUID{box}}, repo.EntityQuery{ParentItemIDs: []uuid.UUID{box}}},
		{"custom field", CSVEntityFilters{IncludeArchived: true, Fields: []CSVFieldFilter{{Name: "Custom", Value: "value"}}}, repo.EntityQuery{IncludeArchived: true, Fields: []repo.FieldQuery{{Name: "Custom", Value: "value"}}}},
		{"descendant tags", CSVEntityFilters{TagIDs: []uuid.UUID{tg}}, repo.EntityQuery{TagIDs: []uuid.UUID{tg}}},
		{"negated tags", CSVEntityFilters{TagIDs: []uuid.UUID{tg}, NegateTags: true}, repo.EntityQuery{TagIDs: []uuid.UUID{tg}, NegateTags: true}},
		{"with photo", CSVEntityFilters{OnlyWithPhoto: true}, repo.EntityQuery{OnlyWithPhoto: true}},
		{"without photo", CSVEntityFilters{OnlyWithoutPhoto: true}, repo.EntityQuery{OnlyWithoutPhoto: true}},
		{"root only", CSVEntityFilters{FilterChildren: true}, repo.EntityQuery{FilterChildren: true}},
		{"asset", CSVEntityFilters{AssetID: box.String()}, repo.EntityQuery{}},
	}
	// Asset capture uses the source asset ID, not the entity UUID.
	for _, e := range all {
		if e.ID == box {
			tests[len(tests)-1].filter.AssetID = e.AssetID.String()
			tests[len(tests)-1].query.AssetID = e.AssetID
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.query
			q.Page, q.PageSize = -1, -1
			expected, err := tRepos.Entities.QueryByGroup(ctx, gid, q)
			require.NoError(t, err)
			r := CSVCaptureRequest{Source: "items", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"id"}, Entities: &tt.filter}
			actual, err := s.Load(ctx, tUser.ID, r)
			require.NoError(t, err)
			ids := []string{}
			for _, e := range expected.Items {
				ids = append(ids, e.ID.String())
			}
			got := []string{}
			for _, row := range actual.Rows {
				got = append(got, row[0].(string))
			}
			require.ElementsMatch(t, ids, got)
		})
	}
	// Each matrix order column is a implemented safe scalar comparison, with ID
	// breaking repeated primary values deterministically in either direction.
	for _, column := range []CSVColumn{"name", "id", "createdAt", "updatedAt", "assetId", "quantity", "insured", "purchasePrice", "location", "archived"} {
		r := CSVCaptureRequest{Source: "items", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{column, "description"}, Ordering: []CSVOrder{{column, "desc"}}}
		actual, err := s.Load(ctx, tUser.ID, r)
		require.NoError(t, err)
		for i := 1; i < len(actual.Rows); i++ {
			require.GreaterOrEqual(t, csvCompare(actual.Rows[i-1][0], actual.Rows[i][0]), 0)
		}
	}
}

func TestCSVInventoryMaintenanceAndText(t *testing.T) {
	gid, _, box, tg := csvInventoryFixture(t)
	ctx := context.Background()
	s := NewCSVInventorySources(tClient, nil)
	_, err := tRepos.MaintEntry.Create(ctx, gid, box, repo.MaintenanceEntryCreate{Name: "Scheduled", ScheduledDate: types.DateFromString("2026-02-01"), Cost: 3.5})
	require.NoError(t, err)
	_, err = tRepos.MaintEntry.Create(ctx, gid, box, repo.MaintenanceEntryCreate{Name: "Completed", CompletedDate: types.DateFromString("2026-01-01"), ScheduledDate: types.DateFromString("2025-12-01"), Cost: 4})
	require.NoError(t, err)
	r := CSVCaptureRequest{Source: "entity-maintenance", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"name", "scheduledDate", "completedDate", "cost", "itemId"}, Hierarchy: &CSVHierarchyContext{Kind: "entity", RootID: box}, Maintenance: &CSVMaintenanceFilters{Status: "both"}}
	out, err := s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 2)
	require.Equal(t, []any{"Completed", "2025-12-01", "2026-01-01", float64(4), box.String()}, out.Rows[0])
	r.Maintenance.Status = "scheduled"
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
	require.Equal(t, "", out.Rows[0][2])
	r.Maintenance.Status = "completed"
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
	require.Equal(t, "Completed", out.Rows[0][0])
	r = CSVCaptureRequest{Source: "tags", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"name", "parentId"}, Text: &CSVTextFilters{Search: "pOw"}}
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Equal(t, [][]any{{"Power", tg.String()}}, out.Rows)
	// Trusted captured ranking, not alphabetical sorting, is preserved by default.
	r = CSVCaptureRequest{Source: "entity-selector", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"id"}, Selector: &CSVSelectorCapture{Kind: "entity", Eligibility: "explicit-set", EligibleIDs: []uuid.UUID{box}}}
	s.boundaries = csvTestBoundary{ids: []uuid.UUID{box}}
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
}

func TestCSVInventoryForeignAndStaleReferences(t *testing.T) {
	gid, _, box, tg := csvInventoryFixture(t)
	other, foreignLoc, foreignBox, foreignTag := csvInventoryFixture(t)
	_ = other
	ctx := context.Background()
	s := NewCSVInventorySources(tClient, nil)
	r := CSVCaptureRequest{Source: "entity-inventory", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"id"}, Hierarchy: &CSVHierarchyContext{Kind: "entity", RootID: foreignLoc, Descendants: true}}
	_, err := s.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	r.Source = "tag-inventory"
	r.Hierarchy = &CSVHierarchyContext{Kind: "tag", RootID: foreignTag}
	_, err = s.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	r.Source = "items"
	r.Hierarchy = nil
	r.Entities = &CSVEntityFilters{ParentIDs: []uuid.UUID{foreignBox}}
	_, err = s.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	r.Entities = nil
	r.Source = "dashboard-recent-items"
	r.Recent = &CSVRecentBoundary{CapturedAt: time.Now(), RecordIDs: []uuid.UUID{foreignBox}}
	s.boundaries = csvTestBoundary{ids: []uuid.UUID{foreignBox}}
	_, err = s.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	// A formerly valid captured ID which was deleted cannot turn into an empty
	// or unrestricted export, even when the trusted provenance check succeeds.
	r.Recent.RecordIDs = []uuid.UUID{box}
	s.boundaries = csvTestBoundary{ids: []uuid.UUID{box}}
	require.NoError(t, tRepos.Entities.DeleteByGroup(ctx, gid, box))
	_, err = s.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	// Corrupt cross-tenant relationships fail before producing any projection.
	r = CSVCaptureRequest{Source: "tags", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"parentId"}}
	require.NoError(t, tClient.Tag.UpdateOneID(tg).SetParentID(foreignTag).Exec(ctx))
	_, err = s.Load(ctx, tUser.ID, r)
	require.ErrorContains(t, err, "invalid hierarchy")
}
