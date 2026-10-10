package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/attachment"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/usergroup"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
)

func csvCaptureMap(t *testing.T, r CSVCaptureRequest) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(r)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

func TestCSVCompleteCatalogueDispatch(t *testing.T) {
	ctx := context.Background()
	gid, _, box, tg := csvInventoryFixture(t)
	svc := NewCSVSources(tClient, csvTestBoundary{ids: []uuid.UUID{}})
	exercised := map[CSVSource]bool{}
	for _, spec := range CSVSourceCatalogue() {
		t.Run(string(spec.Source), func(t *testing.T) {
			r := CSVCaptureRequest{Source: spec.Source, Scope: spec.Scope, Columns: spec.Columns}
			if spec.Scope == CSVCollection {
				r.CollectionID = &gid
			}
			if spec.ContextKind != "" {
				root := box
				if spec.ContextKind == "tag" {
					root = tg
				}
				r.Hierarchy = &CSVHierarchyContext{Kind: spec.ContextKind, RootID: root}
			}
			if spec.SelectorKind != "" {
				r.Selector = &CSVSelectorCapture{Kind: spec.SelectorKind, Eligibility: "explicit-set", EligibleIDs: []uuid.UUID{}}
			}
			if spec.Recent {
				r.Recent = &CSVRecentBoundary{CapturedAt: time.Now(), RecordIDs: []uuid.UUID{}}
			}
			out, err := svc.Load(ctx, tUser.ID, r)
			require.NoError(t, err)
			require.Equal(t, spec.Columns, out.Columns)
			require.NoError(t, WriteCanonicalCSV(io.Discard, out))
			for _, row := range out.Rows {
				require.Len(t, row, len(spec.Columns))
			}
			exercised[spec.Source] = true
		})
	}
	require.Len(t, exercised, len(CSVSourceCatalogue()))
	_, err := svc.Load(ctx, tUser.ID, CSVCaptureRequest{Source: "not-a-source"})
	require.Error(t, err)
}

func TestCSVAdministrativeMetadataIsolation(t *testing.T) {
	ctx := context.Background()
	gid, _, box, _ := csvInventoryFixture(t)
	other, err := tClient.User.Create().SetName("Other").SetEmail(uuid.NewString() + "@test.invalid").Save(ctx)
	require.NoError(t, err)
	require.NoError(t, tClient.Group.UpdateOneID(gid).AddUserIDs(other.ID).Exec(ctx))
	secret := "secret-credential-" + uuid.NewString()
	key, err := tRepos.APIKeys.Create(ctx, other.ID, "other-key", []byte(secret), nil)
	require.NoError(t, err)
	own, err := tRepos.APIKeys.Create(ctx, tUser.ID, "own-key", []byte(uuid.NewString()), nil)
	require.NoError(t, err)
	notifier, err := tRepos.Notifiers.Create(ctx, gid, tUser.ID, repo.NotifierCreate{Name: "mine", URL: secret, IsActive: true})
	require.NoError(t, err)
	_, err = tRepos.Notifiers.Create(ctx, gid, other.ID, repo.NotifierCreate{Name: "theirs", URL: secret})
	require.NoError(t, err)
	photo, err := tClient.Attachment.Create().SetEntityID(box).SetTitle("photo").SetType(attachment.TypePhoto).SetPath(secret).SetMimeType("image/png").Save(ctx)
	require.NoError(t, err)
	_, err = tClient.Attachment.Create().SetEntityID(box).SetTitle("manual").SetType(attachment.TypeManual).SetPath(secret).Save(ctx)
	require.NoError(t, err)
	backup, err := tRepos.Exports.CreateImport(ctx, gid, secret, 123)
	require.NoError(t, err)
	_, err = tRepos.Groups.InvitationCreate(ctx, gid, repo.GroupInvitationCreate{Token: []byte(secret), Uses: 5, ExpiresAt: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	svc := NewCSVSources(tClient, nil)
	for _, source := range []CSVSource{"collection-members", "collection-invitations", "collection-notifiers", "entity-attachments", "entity-media", "backup-history", "account-api-keys"} {
		spec, _ := CSVSourceDefinition(source)
		r := CSVCaptureRequest{Source: source, Scope: spec.Scope, Columns: spec.Columns}
		if spec.Scope == CSVCollection {
			r.CollectionID = &gid
		}
		if spec.ContextKind != "" {
			r.Hierarchy = &CSVHierarchyContext{Kind: "entity", RootID: box}
		}
		out, err := svc.Load(ctx, tUser.ID, r)
		require.NoError(t, err)
		b, err := json.Marshal(out)
		require.NoError(t, err)
		require.NotContains(t, string(b), secret)
		switch source {
		case "collection-notifiers":
			require.Len(t, out.Rows, 1)
			require.Equal(t, notifier.ID.String(), out.Rows[0][0])
		case "entity-media":
			require.Len(t, out.Rows, 1)
			require.Equal(t, photo.ID.String(), out.Rows[0][0])
		case "entity-attachments":
			require.Len(t, out.Rows, 2)
		case "backup-history":
			require.Len(t, out.Rows, 1)
			require.Equal(t, backup.ID.String(), out.Rows[0][0])
		case "account-api-keys":
			require.Contains(t, string(b), own.ID.String())
			require.NotContains(t, string(b), key.ID.String())
		}
		if spec.Scope == CSVCollection {
			_, err = svc.Load(ctx, uuid.New(), r)
			require.Error(t, err)
		}
		for _, column := range []CSVColumn{"token", "hash", "pepper", "path", "url", "thumbnail", "artifactPath"} {
			bad := r
			bad.Columns = []CSVColumn{column}
			_, err := svc.Load(ctx, tUser.ID, bad)
			require.Error(t, err)
		}
	}
	// Membership is not ownership, even for a superuser; the per-group role wins.
	r := CSVCaptureRequest{Source: "collection-members", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"id"}}
	spec, _ := CSVSourceDefinition(r.Source)
	spec.Access = CSVOwner
	require.Error(t, csvSourceAccess(ctx, tRepos, tUser.ID, r, spec))
	require.NoError(t, tClient.UserGroup.Update().Where(usergroup.UserID(tUser.ID), usergroup.GroupID(gid)).SetRole(usergroup.RoleOwner).Exec(ctx))
	require.NoError(t, csvSourceAccess(ctx, tRepos, tUser.ID, r, spec))
	require.NoError(t, tRepos.Groups.RemoveMember(ctx, gid, tUser.ID))
	_, err = svc.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	// Private API-key metadata remains usable without this membership.
	_, err = svc.Load(ctx, tUser.ID, CSVCaptureRequest{Source: "account-api-keys", Scope: CSVAccountPrivate, Columns: []CSVColumn{"id"}})
	require.NoError(t, err)
}

func TestCSVCollectionsAllResultsSelectorsAndAuthorization(t *testing.T) {
	ctx := context.Background()
	u, err := tClient.User.Create().SetName("private").SetEmail(uuid.NewString() + "@test.invalid").Save(ctx)
	require.NoError(t, err)
	ids := []uuid.UUID{}
	for i := 0; i < 111; i++ {
		g, err := tRepos.Groups.GroupCreate(ctx, fmt.Sprintf("CSV collection %03d", i), u.ID)
		require.NoError(t, err)
		ids = append(ids, g.ID)
	}
	r := CSVCaptureRequest{Source: "account-collections", Scope: CSVCrossCollection, Columns: []CSVColumn{"name", "id"}, Text: &CSVTextFilters{Search: "collection"}, Ordering: []CSVOrder{{Column: "name", Direction: "desc"}}}
	svc := NewCSVSources(tClient, nil)
	out, err := svc.Load(ctx, u.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 111)
	require.Equal(t, "CSV collection 110", out.Rows[0][0])
	r.Source = "collection-selector"
	r.Ordering = nil
	r.Selector = &CSVSelectorCapture{Kind: "collection", Eligibility: "explicit-set", EligibleIDs: []uuid.UUID{ids[2], ids[0], ids[1]}, ExcludedIDs: []uuid.UUID{ids[0]}}
	_, err = svc.Load(ctx, u.ID, r)
	require.Error(t, err)
	svc = NewCSVSources(tClient, csvTestBoundary{ids: r.Selector.EligibleIDs})
	out, err = svc.Load(ctx, u.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 2)
	require.Equal(t, ids[2].String(), out.Rows[0][1])
	require.Equal(t, ids[1].String(), out.Rows[1][1])
	require.NoError(t, tRepos.Groups.RemoveMember(ctx, ids[2], u.ID))
	_, err = svc.Load(ctx, u.ID, r)
	require.Error(t, err)
	r.Selector.EligibleIDs = []uuid.UUID{tGroup.ID}
	r.Selector.ExcludedIDs = nil
	svc = NewCSVSources(tClient, csvTestBoundary{ids: r.Selector.EligibleIDs})
	_, err = svc.Load(ctx, u.ID, r)
	require.Error(t, err)
}

func TestCSVHistorySourcePermissionsAndPrivateScope(t *testing.T) {
	ctx := context.Background()
	gid, _, box, _ := csvInventoryFixture(t)
	privateUser, err := tClient.User.Create().SetName("private-history").SetEmail(uuid.NewString() + "@test.invalid").Save(ctx)
	require.NoError(t, err)
	privateCapture := CSVCaptureRequest{Source: "account-api-keys", Scope: CSVAccountPrivate, Columns: []CSVColumn{"id"}}
	privateRow, err := tRepos.CSVExports.Create(ctx, privateUser.ID, nil, csvexport.ScopeAccountPrivate, string(privateCapture.Source), csvCaptureMap(t, privateCapture))
	require.NoError(t, err)
	sharedCapture := CSVCaptureRequest{Source: "items", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"id"}}
	sharedRow, err := tRepos.CSVExports.Create(ctx, tUser.ID, &gid, csvexport.ScopeCollection, string(sharedCapture.Source), csvCaptureMap(t, sharedCapture))
	require.NoError(t, err)
	svc := NewCSVSources(tClient, nil)
	r := CSVCaptureRequest{Source: "private-csv-history", Scope: CSVAccountPrivate, Columns: []CSVColumn{"id", "source", "completedAt", "expiresAt"}}
	out, err := svc.Load(ctx, privateUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
	require.Equal(t, privateRow.ID.String(), out.Rows[0][0])
	require.Equal(t, "", out.Rows[0][2])
	r.Source = "collection-csv-history"
	r.Scope = CSVCollection
	r.CollectionID = &gid
	out, err = svc.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	require.Len(t, out.Rows, 1)
	require.Equal(t, sharedRow.ID.String(), out.Rows[0][0])
	// A user-scoped notifier artifact must not be exposed to another member.
	sharedCapture.Source = "collection-notifiers"
	_, err = tRepos.CSVExports.Create(ctx, tUser.ID, &gid, csvexport.ScopeCollection, string(sharedCapture.Source), csvCaptureMap(t, sharedCapture))
	require.NoError(t, err)
	require.NoError(t, tClient.Group.UpdateOneID(gid).AddUserIDs(privateUser.ID).Exec(ctx))
	_, err = svc.Load(ctx, privateUser.ID, r)
	require.Error(t, err)
	// Stale attachment root denies history rather than dropping the row.
	sharedCapture.Source = "entity-attachments"
	sharedCapture.Hierarchy = &CSVHierarchyContext{Kind: "entity", RootID: uuid.New()}
	_, err = tRepos.CSVExports.Create(ctx, tUser.ID, &gid, csvexport.ScopeCollection, string(sharedCapture.Source), csvCaptureMap(t, sharedCapture))
	require.NoError(t, err)
	_, err = svc.Load(ctx, tUser.ID, r)
	require.Error(t, err)
	bad := CSVCaptureRequest{Source: "entity-media", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"id"}, Hierarchy: &CSVHierarchyContext{Kind: "entity", RootID: box}}
	bad.Hierarchy.RootID = uuid.New()
	_, err = svc.Load(ctx, tUser.ID, bad)
	require.Error(t, err)
}

func TestCSVMetadataAllResultsAndOrderedColumns(t *testing.T) {
	ctx := context.Background()
	gid, _, box, _ := csvInventoryFixture(t)
	u, err := tClient.User.Create().SetName("metadata-owner").SetEmail(uuid.NewString() + "@test.invalid").Save(ctx)
	require.NoError(t, err)
	require.NoError(t, tClient.Group.UpdateOneID(gid).AddUserIDs(u.ID).Exec(ctx))
	for i := 0; i < 103; i++ {
		_, err = tClient.Attachment.Create().SetEntityID(box).SetTitle("same title").SetType(attachment.TypePhoto).SetPath("hidden/path").Save(ctx)
		require.NoError(t, err)
		_, err = tRepos.Notifiers.Create(ctx, gid, u.ID, repo.NotifierCreate{Name: "same name", URL: "hidden credential"})
		require.NoError(t, err)
		_, err = tRepos.APIKeys.Create(ctx, u.ID, "same name", []byte(uuid.NewString()), nil)
		require.NoError(t, err)
		_, err = tRepos.Exports.Create(ctx, gid)
		require.NoError(t, err)
		_, err = tRepos.Groups.InvitationCreate(ctx, gid, repo.GroupInvitationCreate{Token: []byte(uuid.NewString()), Uses: i, ExpiresAt: time.Now().Add(time.Hour)})
		require.NoError(t, err)
	}
	svc := NewCSVSources(tClient, nil)
	for _, source := range []CSVSource{"entity-attachments", "entity-media", "collection-notifiers", "account-api-keys", "backup-history", "collection-invitations"} {
		spec, _ := CSVSourceDefinition(source)
		r := CSVCaptureRequest{Source: source, Scope: spec.Scope, Columns: []CSVColumn{spec.Columns[1], "id"}, Ordering: []CSVOrder{{Column: "id", Direction: "desc"}}}
		if spec.Scope == CSVCollection {
			r.CollectionID = &gid
		}
		if spec.ContextKind != "" {
			r.Hierarchy = &CSVHierarchyContext{Kind: "entity", RootID: box}
		}
		out, err := svc.Load(ctx, u.ID, r)
		require.NoError(t, err)
		require.Equal(t, r.Columns, out.Columns)
		require.Len(t, out.Rows, 103)
		seen := map[any]bool{}
		for i, row := range out.Rows {
			require.False(t, seen[row[1]])
			seen[row[1]] = true
			if i > 0 {
				require.Greater(t, out.Rows[i-1][1].(string), row[1].(string))
			}
		}
	}
}
