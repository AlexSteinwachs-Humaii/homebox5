package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/usergroup"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
)

func TestCSVRetentionCompletionClockAndPolicy(t *testing.T) {
	ctx := context.Background()
	svc, _, row, now, _ := csvWorkerFixture(t)
	policy, err := svc.Retention(ctx, tUser.ID, nil)
	require.NoError(t, err)
	require.Equal(t, 30, policy.Days)
	for _, days := range []int{0, -1, repo.MaxCSVRetentionDays + 1} {
		require.Error(t, svc.SetRetention(ctx, tUser.ID, nil, days))
	}
	require.NoError(t, svc.SetRetention(ctx, tUser.ID, nil, 2))
	*now = now.Add(48 * time.Hour)
	require.NoError(t, svc.RunJob(ctx, row.ID))
	got, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.True(t, now.Equal(*got.CompletedAt))
	require.True(t, now.Add(48*time.Hour).Equal(*got.ExpiresAt))
	require.NoError(t, svc.SetRetention(ctx, tUser.ID, nil, 1))
	unchanged, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, got.ExpiresAt, unchanged.ExpiresAt)
	later, err := svc.Enqueue(ctx, tUser.ID, CSVCaptureRequest{Source: "account-api-keys", Scope: CSVAccountPrivate, Columns: []CSVColumn{"name"}})
	require.NoError(t, err)
	require.NoError(t, svc.RunJob(ctx, later.ID))
	latest, err := svc.db.CSVExport.Get(ctx, later.ID)
	require.NoError(t, err)
	require.True(t, now.Add(24*time.Hour).Equal(*latest.ExpiresAt))
	utc, err := csvExpiry(now.In(time.FixedZone("offset", 3600)), repo.MaxCSVRetentionDays)
	require.NoError(t, err)
	require.True(t, utc.Equal(now.Add(time.Duration(repo.MaxCSVRetentionDays)*24*time.Hour)))
}

func TestCSVRetentionCurrentOwnerOnly(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _ := csvWorkerFixture(t)
	group, err := svc.db.Group.Create().SetName("policy").Save(ctx)
	require.NoError(t, err)
	gid := group.ID
	_, err = svc.db.UserGroup.Create().SetGroupID(gid).SetUserID(tUser.ID).SetRole(usergroup.RoleUser).Save(ctx)
	require.NoError(t, err)
	_, err = svc.Retention(ctx, tUser.ID, &gid)
	require.ErrorIs(t, err, ErrCSVAccess)
	require.ErrorIs(t, svc.SetRetention(ctx, tUser.ID, &gid, 3), ErrCSVAccess)
	require.NoError(t, svc.db.UserGroup.Update().Where(usergroup.UserID(tUser.ID), usergroup.GroupID(gid)).SetRole(usergroup.RoleOwner).Exec(ctx))
	policy, err := svc.Retention(ctx, tUser.ID, &gid)
	require.NoError(t, err)
	require.Equal(t, 30, policy.Days)
	require.NoError(t, svc.SetRetention(ctx, tUser.ID, &gid, 3))
	require.NoError(t, svc.db.UserGroup.Update().Where(usergroup.UserID(tUser.ID), usergroup.GroupID(gid)).SetRole(usergroup.RoleUser).Exec(ctx))
	require.ErrorIs(t, svc.SetRetention(ctx, tUser.ID, &gid, 4), ErrCSVAccess)
	_, err = svc.db.UserGroup.Delete().Where(usergroup.UserID(tUser.ID), usergroup.GroupID(gid)).Exec(ctx)
	require.NoError(t, err)
	_, err = svc.Retention(ctx, tUser.ID, &gid)
	require.ErrorIs(t, err, ErrCSVAccess)
	require.NoError(t, svc.SetRetention(ctx, tUser.ID, nil, 5))
}

func TestCSVCleanupExpirationAndFailures(t *testing.T) {
	ctx := context.Background()
	svc, _, row, now, objects := csvWorkerFixture(t)
	require.NoError(t, svc.RunJob(ctx, row.ID))
	got, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	calls := 0
	failBlob := true
	svc.remove = func(_ context.Context, key string) error {
		calls++
		if failBlob {
			return errors.New("blob offline")
		}
		delete(*objects, key) // Missing objects are also successful deletions.
		return nil
	}
	*now = got.ExpiresAt.Add(-time.Nanosecond)
	require.NoError(t, svc.Cleanup(ctx))
	require.Zero(t, calls)
	*now = *got.ExpiresAt
	require.ErrorContains(t, svc.Cleanup(ctx), "blob offline")
	got, err = svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusCleanupPending, got.Status)
	require.NotEmpty(t, got.CleanupError)
	require.Contains(t, *objects, row.BlobKey)
	require.NoError(t, svc.Cleanup(ctx))
	require.Equal(t, 1, calls)
	*now = now.Add(csvRetryDelay)
	failBlob = false
	failSQL := true
	svc.db.CSVExport.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			if m.Op().Is(ent.OpDeleteOne) && failSQL {
				return nil, errors.New("SQL delete offline")
			}
			return next.Mutate(ctx, m)
		})
	})
	require.ErrorContains(t, svc.Cleanup(ctx), "SQL delete offline")
	require.Empty(t, *objects)
	got, err = svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, row.BlobKey, got.BlobKey)
	failSQL = false
	*now = now.Add(csvRetryDelay)
	require.NoError(t, svc.Cleanup(ctx))
	_, err = svc.db.CSVExport.Get(ctx, row.ID)
	require.True(t, ent.IsNotFound(err))
	require.NoError(t, svc.Cleanup(ctx))
	require.NoError(t, svc.RunJob(ctx, row.ID))
	require.NoError(t, svc.Delete(ctx, tUser.ID, nil, row.ID))
}

func TestCSVCleanupAuthorizedDeletionFencesWorker(t *testing.T) {
	ctx := context.Background()
	svc, _, row, _, objects := csvWorkerFixture(t)
	// A running job with a partial blob can be deleted before completion.
	(*objects)[row.BlobKey] = []byte("partial")
	require.NoError(t, svc.Delete(ctx, tUser.ID, nil, row.ID))
	svc.remove = func(ctx context.Context, key string) error { delete(*objects, key); return nil }
	require.NoError(t, svc.RunJob(ctx, row.ID))
	require.NoError(t, svc.Cleanup(ctx))
	require.Empty(t, *objects)
	require.NoError(t, svc.Delete(ctx, tUser.ID, nil, row.ID))
	require.NoError(t, svc.RunJob(ctx, uuid.New()))
}

func TestCSVCleanupConcurrentDeletionAndDelivery(t *testing.T) {
	ctx := context.Background()
	svc, _, row, now, _ := csvWorkerFixture(t)
	require.NoError(t, svc.Delete(ctx, tUser.ID, nil, row.ID))
	svc.remove = func(context.Context, string) error { return nil }
	results := make(chan error, 12)
	for i := 0; i < 4; i++ {
		go func() { results <- svc.Cleanup(ctx) }()
		go func() { results <- svc.Delete(ctx, tUser.ID, nil, row.ID) }()
		go func() { results <- svc.RunJob(ctx, row.ID) }()
	}
	for i := 0; i < 12; i++ {
		// SQLite may reject competing writers. All such attempts remain recoverable.
		if err := <-results; err != nil {
			t.Logf("recoverable concurrent attempt: %v", err)
		}
	}
	*now = now.Add(csvRetryDelay)
	require.NoError(t, svc.Cleanup(ctx))
	_, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.True(t, ent.IsNotFound(err))
	require.NoError(t, svc.RunJob(ctx, row.ID))
}
