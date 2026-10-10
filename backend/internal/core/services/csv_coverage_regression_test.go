package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// This independent list is deliberately not derived from the implementation:
// removing or silently remapping a listed surface must fail verification.
func TestCSVSurfaceContractRegression(t *testing.T) {
	expected := map[CSVSource]CSVAdapter{
		"items": CSVEntities, "locations": CSVEntities, "dashboard-recent-items": CSVEntities,
		"entity-inventory": CSVEntities, "tag-inventory": CSVEntities, "entity-hierarchy": CSVEntities,
		"item-selector": CSVEntities, "location-selector": CSVEntities, "entity-selector": CSVEntities,
		"tags": CSVTags, "tag-hierarchy": CSVTags, "tag-selector": CSVTags,
		"entity-types": CSVEntityTypes, "entity-type-selector": CSVEntityTypes,
		"templates": CSVTemplates, "template-selector": CSVTemplates,
		"maintenance": CSVMaintenance, "entity-maintenance": CSVMaintenance,
		"collection-members": CSVMembers, "collection-invitations": CSVInvitations,
		"collection-notifiers": CSVNotifiers, "entity-attachments": CSVAttachments, "entity-media": CSVAttachments,
		"backup-history": CSVBackups, "collection-csv-history": CSVArtifacts, "private-csv-history": CSVArtifacts,
		"account-api-keys": CSVAPIKeys, "account-collections": CSVCollections, "collection-selector": CSVCollections,
	}
	require.Len(t, CSVSourceCatalogue(), len(expected))
	gid, _, box, tag := csvInventoryFixture(t)
	sources := NewCSVSources(tClient, csvTestBoundary{ids: []uuid.UUID{}})
	ctx := context.Background()
	for source, adapter := range expected {
		t.Run(string(source), func(t *testing.T) {
			spec, ok := CSVSourceDefinition(source)
			require.True(t, ok)
			require.Equal(t, adapter, spec.Adapter)
			r := CSVCaptureRequest{Source: source, Scope: spec.Scope, Columns: []CSVColumn{spec.Columns[len(spec.Columns)-1], spec.Columns[0]}}
			if spec.Scope == CSVCollection {
				r.CollectionID = &gid
			}
			if spec.ContextKind != "" {
				root := box
				if spec.ContextKind == "tag" {
					root = tag
				}
				r.Hierarchy = &CSVHierarchyContext{Kind: spec.ContextKind, RootID: root}
			}
			if spec.SelectorKind != "" {
				r.Selector = &CSVSelectorCapture{Kind: spec.SelectorKind, Eligibility: "explicit-set", EligibleIDs: []uuid.UUID{}}
			}
			if spec.Recent {
				r.Recent = &CSVRecentBoundary{CapturedAt: time.Now(), RecordIDs: []uuid.UUID{}}
			}
			// Every supported ordering is exercised through the actual adapter. Filters
			// use a guaranteed non-match to ensure adapters do not ignore predicates.
			for _, column := range spec.OrderColumns {
				for _, direction := range []string{"asc", "desc"} {
					q := r
					q.Ordering = []CSVOrder{{Column: column, Direction: direction}}
					out, err := sources.Load(ctx, tUser.ID, q)
					require.NoError(t, err, "order %s %s", column, direction)
					require.Equal(t, r.Columns, out.Columns)
					if spec.SelectorKind != "" || spec.Recent {
						require.Empty(t, out.Rows, "empty boundary must never become unrestricted")
					}
				}
			}
			q := r
			if spec.EntityFilter {
				q.Entities = &CSVEntityFilters{Search: uuid.NewString()}
			}
			if spec.TextFilter {
				q.Text = &CSVTextFilters{Search: uuid.NewString()}
			}
			if spec.MaintenanceFilter {
				q.Maintenance = &CSVMaintenanceFilters{Status: "completed"}
			}
			out, err := sources.Load(ctx, tUser.ID, q)
			require.NoError(t, err)
			if spec.EntityFilter || spec.TextFilter || spec.MaintenanceFilter {
				require.Empty(t, out.Rows)
			}
			for _, forbidden := range []CSVColumn{"actions", "token", "blobKey", "hashedKey"} {
				q = r
				q.Columns = []CSVColumn{forbidden}
				_, err := sources.Load(ctx, tUser.ID, q)
				require.Error(t, err)
			}
			q = r
			q.Ordering = []CSVOrder{{Column: "token", Direction: "asc"}}
			_, err = sources.Load(ctx, tUser.ID, q)
			require.Error(t, err)
			q = r
			q.Columns = []CSVColumn{spec.Columns[0], spec.Columns[0]}
			_, err = sources.Load(ctx, tUser.ID, q)
			require.Error(t, err)
			if spec.ContextKind != "" {
				q = r
				q.Hierarchy = &CSVHierarchyContext{Kind: spec.ContextKind, RootID: uuid.New()}
				_, err = sources.Load(ctx, tUser.ID, q)
				require.Error(t, err, "stale context must fail closed")
			}
			if spec.SelectorKind != "" || spec.Recent {
				denied := NewCSVSources(tClient, csvTestBoundary{deny: true})
				_, err = denied.Load(ctx, tUser.ID, r)
				require.Error(t, err, "boundary authority is mandatory even for empty sets")
			}
		})
	}
}
