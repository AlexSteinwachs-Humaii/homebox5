package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/usergroup"
)

func TestCSVLifecyclePrivateDownloadExpiryDeletion(t *testing.T) {
	ctx := context.Background()
	svc, src, row, now, objects := csvWorkerFixture(t)
	svc.read = func(_ context.Context, key string) ([]byte, error) { return (*objects)[key], nil }
	require.NoError(t, svc.RunJob(ctx, row.ID))
	out, err := svc.Status(ctx, tUser.ID, nil, row.ID)
	require.NoError(t, err)
	require.Equal(t, "none", out.RetryMode)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "capture")
	require.NotContains(t, string(raw), row.BlobKey)
	history, err := svc.History(ctx, tUser.ID, nil)
	require.NoError(t, err)
	require.Len(t, history, 1)
	data, err := svc.Download(ctx, tUser.ID, nil, row.ID)
	require.NoError(t, err)
	require.Equal(t, (*objects)[row.BlobKey], data)
	// Formerly permitted records or values disappearing must deny old bytes.
	original := src.data.Rows
	src.data.Rows = src.data.Rows[:1]
	_, err = svc.Download(ctx, tUser.ID, nil, row.ID)
	require.ErrorIs(t, err, ErrCSVAccess)
	src.data.Rows = original
	// Expiry is exact and independent of cleanup progress.
	*now = out.ExpiresAt.Add(-time.Nanosecond)
	_, err = svc.Download(ctx, tUser.ID, nil, row.ID)
	require.NoError(t, err)
	*now = *out.ExpiresAt
	_, err = svc.Download(ctx, tUser.ID, nil, row.ID)
	require.ErrorIs(t, err, ErrCSVExpired)
	// Download and client failures never mutate generation status.
	got, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusCompleted, got.Status)
	require.NoError(t, svc.Delete(ctx, tUser.ID, nil, row.ID))
	require.NoError(t, svc.Delete(ctx, tUser.ID, nil, row.ID))
	_, err = svc.Download(ctx, tUser.ID, nil, row.ID)
	require.ErrorIs(t, err, ErrCSVNotReady)
	require.NoError(t, svc.RunJob(ctx, row.ID))
	got, err = svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusCleanupPending, got.Status)
	require.NoError(t, svc.db.CSVExport.DeleteOneID(row.ID).Exec(ctx))
	require.NoError(t, svc.Delete(ctx, tUser.ID, nil, row.ID))
}

func TestCSVLifecycleRechecksPrivateOwnerStorageAndRetry(t *testing.T) {
	ctx := context.Background()
	svc, src, row, _, _ := csvWorkerFixture(t)
	other, err := svc.db.User.Create().SetName("other").SetEmail(uuid.NewString() + "@test.invalid").Save(ctx)
	require.NoError(t, err)
	_, err = svc.Status(ctx, other.ID, nil, row.ID)
	require.ErrorIs(t, err, ErrCSVAccess)
	list, err := svc.History(ctx, other.ID, nil)
	require.NoError(t, err)
	require.Empty(t, list)
	require.ErrorIs(t, svc.RetryArtifact(ctx, other.ID, nil, row.ID), ErrCSVAccess)
	src.err = errors.New("secret sql or revoked source")
	require.Error(t, svc.RunJob(ctx, row.ID))
	src.err = nil
	status, err := svc.Status(ctx, tUser.ID, nil, row.ID)
	require.NoError(t, err)
	require.Equal(t, "source", status.FailureCode)
	require.Equal(t, "requester-explicit", status.RetryMode)
	require.NoError(t, svc.RetryArtifact(ctx, tUser.ID, nil, row.ID))
	require.NoError(t, svc.RunJob(ctx, row.ID))
	svc.read = func(context.Context, string) ([]byte, error) { return nil, errors.New("storage secret") }
	_, err = svc.Download(ctx, tUser.ID, nil, row.ID)
	require.ErrorIs(t, err, ErrCSVStorage)
	src.err = errors.New("revoked")
	_, err = svc.Status(ctx, tUser.ID, nil, row.ID)
	require.ErrorIs(t, err, ErrCSVAccess)
	require.ErrorIs(t, svc.Delete(ctx, tUser.ID, nil, row.ID), ErrCSVAccess)
	list, err = svc.History(ctx, tUser.ID, nil)
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestCSVLifecycleCurrentMembershipAndScope(t *testing.T) {
	ctx := context.Background()
	gid, _, _, _ := csvInventoryFixture(t)
	svc := NewCSVExportService(tClient, tRepos, nil)
	r := CSVCaptureRequest{Source: "collection-members", Scope: CSVCollection, CollectionID: &gid, Columns: []CSVColumn{"name"}}
	row, err := svc.Enqueue(ctx, tUser.ID, r)
	require.NoError(t, err)
	defer func() { _ = tClient.CSVExport.DeleteOneID(row.ID).Exec(ctx) }()
	other, err := tClient.User.Create().SetName("member").SetEmail(uuid.NewString() + "@test.invalid").Save(ctx)
	require.NoError(t, err)
	require.NoError(t, tClient.Group.UpdateOneID(gid).AddUserIDs(other.ID).Exec(ctx))
	_, err = svc.Status(ctx, other.ID, &gid, row.ID)
	// Member metadata is member-visible in the source API, not owner-only.
	require.NoError(t, err)
	history, err := svc.History(ctx, other.ID, &gid)
	require.NoError(t, err)
	require.Len(t, history, 1)
	// Changing role must preserve this source's existing member semantics.
	_, err = tClient.UserGroup.Update().Where(usergroup.UserID(tUser.ID), usergroup.GroupID(gid)).SetRole(usergroup.RoleUser).Save(ctx)
	require.NoError(t, err)
	_, err = svc.Status(ctx, tUser.ID, &gid, row.ID)
	require.NoError(t, err)
	_, err = tClient.UserGroup.Update().Where(usergroup.UserID(tUser.ID), usergroup.GroupID(gid)).SetRole(usergroup.RoleOwner).Save(ctx)
	require.NoError(t, err)
	_, err = svc.Status(ctx, tUser.ID, &gid, row.ID)
	require.NoError(t, err)
	// Private API never falls back to the artifact's collection.
	_, err = svc.Status(ctx, tUser.ID, nil, row.ID)
	require.ErrorIs(t, err, ErrCSVAccess)
	_, err = tClient.UserGroup.Delete().Where(usergroup.UserID(tUser.ID), usergroup.GroupID(gid)).Exec(ctx)
	require.NoError(t, err)
	_, err = svc.Status(ctx, tUser.ID, &gid, row.ID)
	require.ErrorIs(t, err, ErrCSVAccess)
	_, err = svc.History(ctx, tUser.ID, &gid)
	require.ErrorIs(t, err, ErrCSVAccess)
	require.ErrorIs(t, svc.Delete(ctx, tUser.ID, &gid, row.ID), ErrCSVAccess)
}
