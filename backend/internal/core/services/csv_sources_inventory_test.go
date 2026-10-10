package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/data/types"
)

type csvTestBoundary struct {
	ids  []uuid.UUID
	deny bool
}

func (b csvTestBoundary) AuthorizeCSVBoundary(_ context.Context, _ uuid.UUID, r CSVCaptureRequest) error {
	if b.deny {
		return errors.New("untrusted boundary")
	}
	ids := b.ids
	if r.Selector != nil {
		if len(ids) != len(r.Selector.EligibleIDs) {
			return errors.New("eligibility mismatch")
		}
		for i, id := range ids {
			if r.Selector.EligibleIDs[i] != id {
				return errors.New("eligibility mismatch")
			}
		}
	}
	if r.Recent != nil {
		if r.Recent.CapturedAt.After(time.Now()) {
			return errors.New("future boundary")
		}
		if len(ids) != len(r.Recent.RecordIDs) {
			return errors.New("recent mismatch")
		}
		for i, id := range ids {
			if r.Recent.RecordIDs[i] != id {
				return errors.New("recent mismatch")
			}
		}
	}
	return nil
}

func csvInventoryFixture(t *testing.T) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	g, err := tRepos.Groups.GroupCreate(ctx, "csv-inventory-"+uuid.NewString(), uuid.Nil)
	require.NoError(t, err)
	require.NoError(t, tClient.Group.UpdateOneID(g.ID).AddUserIDs(tUser.ID).Exec(ctx))
	itemType, err := tRepos.EntityTypes.GetDefault(ctx, g.ID, false)
	require.NoError(t, err)
	locType, err := tRepos.EntityTypes.GetDefault(ctx, g.ID, true)
	require.NoError(t, err)
	loc, err := tRepos.Entities.Create(ctx, g.ID, repo.EntityCreate{Name: "Garage", EntityTypeID: locType.ID})
	require.NoError(t, err)
	box, err := tRepos.Entities.Create(ctx, g.ID, repo.EntityCreate{Name: "Box", EntityTypeID: itemType.ID, ParentID: loc.ID})
	require.NoError(t, err)
	tg, err := tRepos.Tags.Create(ctx, g.ID, repo.TagCreate{Name: "Tools"})
	require.NoError(t, err)
	childTag, err := tRepos.Tags.Create(ctx, g.ID, repo.TagCreate{Name: "Power", ParentID: tg.ID})
	require.NoError(t, err)
	for i := 0; i < 121; i++ {
		_, err := tRepos.Entities.Create(ctx, g.ID, repo.EntityCreate{Name: "Repeated", Description: "match", Quantity: float64(i + 1), ParentID: box.ID, EntityTypeID: itemType.ID, TagIDs: []uuid.UUID{childTag.ID}})
		require.NoError(t, err)
	}
	return g.ID, loc.ID, box.ID, tg.ID
}

func TestCSVInventoryAllResultsAndHierarchy(t *testing.T) {
	gid, loc, box, tg := csvInventoryFixture(t)
	ctx := context.Background()
	s := NewCSVInventorySources(tClient, nil)
	r := CSVCaptureRequest{Source: "items", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"quantity", "id", "parentId", "location", "tags"}, Entities: &CSVEntityFilters{Search: "match"}, Ordering: []CSVOrder{{"quantity", "desc"}}}
	out, err := s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Equal(t, r.Columns, out.Columns)
	require.Len(t, out.Rows, 121)
	seen := map[any]bool{}
	for i, row := range out.Rows {
		require.Equal(t, float64(121-i), row[0])
		require.False(t, seen[row[1]])
		seen[row[1]] = true
		require.Equal(t, box.String(), row[2])
		require.Equal(t, "Garage", row[3])
		require.Equal(t, "Power", row[4])
	}
	r.Ordering = []CSVOrder{{"name", "asc"}}
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	for i := 1; i < len(out.Rows); i++ {
		require.Less(t, out.Rows[i-1][1].(string), out.Rows[i][1].(string))
	}
	r.Source = "entity-hierarchy"
	r.Hierarchy = &CSVHierarchyContext{Kind: "entity", RootID: loc, Descendants: true}
	r.Entities = nil
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 122)
	r.Hierarchy.Descendants = false
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
	r.Source = "tag-inventory"
	r.Hierarchy = &CSVHierarchyContext{Kind: "tag", RootID: tg, Descendants: false}
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Empty(t, out.Rows)
	r.Hierarchy.Descendants = true
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 121)
	r.Source = "items"
	r.Hierarchy = nil
	r.Entities = &CSVEntityFilters{TagIDs: []uuid.UUID{tg}}
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 121)
	r.Entities.NegateTags = true
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
}

func TestCSVInventoryBoundariesAndAuthorization(t *testing.T) {
	gid, loc, box, _ := csvInventoryFixture(t)
	ctx := context.Background()
	s := NewCSVInventorySources(tClient, csvTestBoundary{ids: []uuid.UUID{box}})
	r := CSVCaptureRequest{Source: "item-selector", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"id"}, Selector: &CSVSelectorCapture{Kind: "item", Eligibility: "explicit-set", EligibleIDs: []uuid.UUID{box}, ExcludedIDs: []uuid.UUID{box}}}
	out, err := s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Empty(t, out.Rows)
	r.Selector.ExcludedIDs = nil
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Equal(t, box.String(), out.Rows[0][0])
	r.Entities = &CSVEntityFilters{Search: "does not match"}
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Empty(t, out.Rows)
	r.Entities = nil
	r.Selector.EligibleIDs = []uuid.UUID{loc}
	_, err = s.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	s.boundaries = csvTestBoundary{ids: []uuid.UUID{loc}}
	_, err = s.Load(ctx, tUser.ID, r)
	require.Error(t, err) // wrong source kind even with trusted capture
	r.Source = "dashboard-recent-items"
	r.Selector = nil
	r.Recent = &CSVRecentBoundary{CapturedAt: time.Now(), RecordIDs: []uuid.UUID{box}}
	s.boundaries = csvTestBoundary{ids: []uuid.UUID{box}}
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
	r.Recent.RecordIDs = []uuid.UUID{}
	s.boundaries = csvTestBoundary{ids: []uuid.UUID{}}
	out, err = s.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Empty(t, out.Rows)
	s.boundaries = nil
	_, err = s.Load(ctx, tUser.ID, r)
	require.ErrorContains(t, err, "trusted boundary")
	r.Source = "items"
	r.Recent = nil
	_, err = s.Load(ctx, uuid.New(), r)
	require.Error(t, err)
	r.Entities = &CSVEntityFilters{ParentIDs: []uuid.UUID{uuid.New()}}
	_, err = s.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	r.Entities = &CSVEntityFilters{TagIDs: []uuid.UUID{uuid.New()}}
	_, err = s.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	r.Entities = nil
	require.NoError(t, tRepos.Groups.RemoveMember(ctx, gid, tUser.ID))
	_, err = s.Load(ctx, tUser.ID, r)
	require.Error(t, err)
}

func TestCSVInventoryCoverage(t *testing.T) {
	gid, loc, box, tg := csvInventoryFixture(t)
	ctx := context.Background()
	_, err := tClient.EntityTemplate.Create().SetName("Template").SetDescription("example").SetGroupID(gid).Save(ctx)
	require.NoError(t, err)
	_, err = tRepos.MaintEntry.Create(ctx, gid, box, repo.MaintenanceEntryCreate{Name: "Service", ScheduledDate: types.DateFromString("2026-01-02"), Cost: 12.5})
	require.NoError(t, err)
	supported := 0
	for _, spec := range CSVSourceCatalogue() {
		if spec.Adapter != CSVEntities && spec.Adapter != CSVTags && spec.Adapter != CSVTemplates && spec.Adapter != CSVEntityTypes && spec.Adapter != CSVMaintenance {
			continue
		}
		supported++
		t.Run(string(spec.Source), func(t *testing.T) {
			r := CSVCaptureRequest{Source: spec.Source, Scope: spec.Scope, CollectionID: &gid, Columns: spec.Columns}
			switch spec.ContextKind {
			case "entity":
				r.Hierarchy = &CSVHierarchyContext{Kind: "entity", RootID: loc, Descendants: spec.Adapter == CSVEntities}
				if spec.Adapter == CSVMaintenance {
					r.Hierarchy.RootID = box
				}
			case "tag":
				r.Hierarchy = &CSVHierarchyContext{Kind: "tag", RootID: tg, Descendants: true}
			}
			ids := []uuid.UUID{}
			if spec.SelectorKind != "" {
				r.Selector = &CSVSelectorCapture{Kind: spec.SelectorKind, Eligibility: "explicit-set", EligibleIDs: ids}
			}
			if spec.Recent {
				r.Recent = &CSVRecentBoundary{CapturedAt: time.Now(), RecordIDs: ids}
			}
			out, err := NewCSVInventorySources(tClient, csvTestBoundary{ids: ids}).Load(ctx, tUser.ID, r)
			require.NoError(t, err)
			require.Equal(t, spec.Columns, out.Columns)
			for _, row := range out.Rows {
				require.Len(t, row, len(spec.Columns))
			}
		})
	}
	require.Equal(t, 18, supported)
}

func TestCSVInventoryInvalidGraphs(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	require.Error(t, csvValidateGraph(map[uuid.UUID]uuid.UUID{a: b}))
	require.Error(t, csvValidateGraph(map[uuid.UUID]uuid.UUID{a: b, b: a}))
	graph := map[uuid.UUID]uuid.UUID{a: uuid.Nil}
	p := a
	for i := 0; i < 1000; i++ {
		n := uuid.New()
		graph[n] = p
		p = n
	}
	require.NoError(t, csvValidateGraph(graph))
	require.True(t, csvWithin(graph, p, a, true))
	gid, _, box, _ := csvInventoryFixture(t)
	ctx := context.Background()
	require.NoError(t, tClient.Entity.UpdateOneID(box).SetParentID(box).Exec(ctx))
	_, err := NewCSVInventorySources(tClient, nil).Load(ctx, tUser.ID, CSVCaptureRequest{Source: "items", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"id"}})
	require.ErrorContains(t, err, "invalid hierarchy")
}
