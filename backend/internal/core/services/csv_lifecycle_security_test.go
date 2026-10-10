package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/usergroup"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
)

func TestCSVLifecycleTamperedKeysCaptureAndScope(t *testing.T) {
	ctx := context.Background()
	svc, _, row, _, _ := csvWorkerFixture(t)
	original := *row
	row.BlobKey = "csv/" + uuid.NewString() + "/" + row.ID.String() + ".csv"
	_, err := svc.authorizeArtifact(ctx, tUser.ID, row)
	require.ErrorIs(t, err, ErrCSVAccess)
	*row = original
	row.Source = "account-collections"
	_, err = svc.authorizeArtifact(ctx, tUser.ID, row)
	require.ErrorIs(t, err, ErrCSVAccess)
	*row = original
	row.Capture = map[string]interface{}{"source": "account-api-keys", "scope": "account-private", "columns": []string{"token"}}
	_, err = svc.authorizeArtifact(ctx, tUser.ID, row)
	require.ErrorIs(t, err, ErrCSVAccess)
	*row = original
	foreign := uuid.New()
	row.GroupID = &foreign
	_, err = svc.authorizeArtifact(ctx, tUser.ID, row)
	require.ErrorIs(t, err, ErrCSVAccess)
	*row = original
	row.Capture = map[string]interface{}{"source": "account-api-keys", "scope": "account-private", "columns": []string{"name"}, "roles": []string{"owner"}}
	_, err = svc.authorizeArtifact(ctx, tUser.ID, row)
	require.ErrorIs(t, err, ErrCSVAccess)
}

func TestCSVLifecycleCrossCollectionRevocationAndPrivateIsolation(t *testing.T) {
	ctx := context.Background()
	gid, _, _, _ := csvInventoryFixture(t)
	repos := repo.New(tClient, nil, config.Storage{ConnString: "file://" + t.TempDir(), PrefixPath: "artifacts"}, "", config.Thumbnail{})
	svc := NewCSVExportService(tClient, repos, nil)
	row, err := svc.Enqueue(ctx, tUser.ID, CSVCaptureRequest{Source: "account-collections", Scope: CSVCrossCollection, Columns: []CSVColumn{"id", "name"}})
	require.NoError(t, err)
	defer func() { _ = tClient.CSVExport.DeleteOneID(row.ID).Exec(ctx) }()
	require.NoError(t, svc.RunJob(ctx, row.ID))
	data, err := svc.Download(ctx, tUser.ID, nil, row.ID)
	require.NoError(t, err)
	require.Contains(t, string(data), gid.String())
	groupHistory, err := svc.History(ctx, tUser.ID, &gid)
	require.NoError(t, err)
	for _, entry := range groupHistory {
		require.NotEqual(t, row.ID, entry.ID)
	}
	// Removing one authorized collection changes the accessible dataset. The
	// retained CSV must not preserve access to that removed collection's rows.
	_, err = tClient.UserGroup.Delete().Where(usergroup.UserID(tUser.ID), usergroup.GroupID(gid)).Exec(ctx)
	require.NoError(t, err)
	_, err = svc.Download(ctx, tUser.ID, nil, row.ID)
	require.ErrorIs(t, err, ErrCSVAccess)
	require.NoError(t, svc.Delete(ctx, tUser.ID, nil, row.ID))
}

func TestCSVLifecycleCapturedBoundaryIsRechecked(t *testing.T) {
	ctx := context.Background()
	gid, _, box, _ := csvInventoryFixture(t)
	repos := repo.New(tClient, nil, config.Storage{ConnString: "file://" + t.TempDir(), PrefixPath: "artifacts"}, "", config.Thumbnail{})
	svc := NewCSVExportService(tClient, repos, csvTestBoundary{ids: []uuid.UUID{box}})
	row, err := svc.Enqueue(ctx, tUser.ID, CSVCaptureRequest{Source: "entity-selector", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"name"}, Selector: &CSVSelectorCapture{Kind: "entity", Eligibility: "explicit-set", EligibleIDs: []uuid.UUID{box}}})
	require.NoError(t, err)
	defer func() { _ = tClient.CSVExport.DeleteOneID(row.ID).Exec(ctx) }()
	require.NoError(t, svc.RunJob(ctx, row.ID))
	_, err = svc.Download(ctx, tUser.ID, &gid, row.ID)
	require.NoError(t, err)
	svc.sources = NewCSVSources(tClient, csvTestBoundary{deny: true})
	_, err = svc.Download(ctx, tUser.ID, &gid, row.ID)
	require.ErrorIs(t, err, ErrCSVAccess)
	_, err = svc.Status(ctx, tUser.ID, &gid, row.ID)
	require.ErrorIs(t, err, ErrCSVAccess)
	require.ErrorIs(t, svc.Delete(ctx, tUser.ID, &gid, row.ID), ErrCSVAccess)
}

func TestCSVLifecycleRechecksAfterStorageRead(t *testing.T) {
	ctx := context.Background()
	for _, cause := range []string{"expiry", "deletion", "permission"} {
		t.Run(cause, func(t *testing.T) {
			svc, src, row, now, objects := csvWorkerFixture(t)
			require.NoError(t, svc.RunJob(ctx, row.ID))
			completed, err := svc.Status(ctx, tUser.ID, nil, row.ID)
			require.NoError(t, err)
			svc.read = func(_ context.Context, key string) ([]byte, error) {
				switch cause {
				case "expiry":
					*now = *completed.ExpiresAt
				case "deletion":
					require.NoError(t, svc.Delete(ctx, tUser.ID, nil, row.ID))
				case "permission":
					src.err = ErrCSVAccess
				}
				return (*objects)[key], nil
			}
			data, err := svc.Download(ctx, tUser.ID, nil, row.ID)
			require.Error(t, err)
			require.Nil(t, data)
		})
	}
}
