package services

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/usergroup"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
	"gocloud.dev/blob"
)

// Use independent databases, real typed adapters and filesystem blobs. PostgreSQL
// absence is an explicit skip, not evidence that the second dialect passed.
func csvIntegrationClient(t *testing.T, driver string) *ent.Client {
	t.Helper()
	if driver == "sqlite3" {
		c, err := ent.Open(driver, "file:"+uuid.NewString()+"?mode=memory&cache=shared&_fk=1&_time_format=sqlite")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c.Close()) })
		return c
	}
	dsn := os.Getenv("CSV_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("set CSV_TEST_POSTGRES_URL (with CREATEDB) for PostgreSQL lifecycle verification")
	}
	admin, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	name := "csv_lifecycle_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.Exec("DROP DATABASE " + name)
		require.NoError(t, err)
		require.NoError(t, admin.Close())
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	require.NoError(t, err)
	c := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { require.NoError(t, c.Close()) })
	return c
}

func TestCSVRealStorageLifecycleBothDialects(t *testing.T) {
	for _, driver := range []string{"sqlite3", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			ctx := context.Background()
			db := csvIntegrationClient(t, driver)
			require.NoError(t, db.Schema.Create(ctx))
			u, err := db.User.Create().SetName("Requester").SetEmail("owner@test.invalid").Save(ctx)
			require.NoError(t, err)
			other, err := db.User.Create().SetName("Other").SetEmail("other@test.invalid").Save(ctx)
			require.NoError(t, err)
			repos := repo.New(db, nil, config.Storage{ConnString: "file://" + t.TempDir(), PrefixPath: "artifacts"}, "", config.Thumbnail{})
			svc := NewCSVExportService(db, repos, nil)
			now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
			svc.now = func() time.Time { return now }
			_, err = repos.APIKeys.Create(ctx, u.ID, "雪,\"quoted\"\n\t=1", []byte("secret-hash"), nil)
			require.NoError(t, err)
			capture := CSVCaptureRequest{Source: "account-api-keys", Scope: CSVAccountPrivate, Columns: []CSVColumn{"name", "createdAt"}}
			row, err := svc.Enqueue(ctx, u.ID, capture)
			require.NoError(t, err)
			// Publication metadata fails after a real upload. A new service must recover
			// solely from durable SQL intent, without orphaning the uploaded object.
			fail := true
			db.CSVExport.Use(func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
					if cm, ok := m.(*ent.CSVExportMutation); ok {
						status, set := cm.Status()
						if fail && set && status == csvexport.StatusCompleted {
							return nil, errors.New("injected completion failure")
						}
					}
					return next.Mutate(ctx, m)
				})
			})
			require.ErrorContains(t, svc.RunJob(ctx, row.ID), "injected completion failure")
			pending, err := db.CSVExport.Get(ctx, row.ID)
			require.NoError(t, err)
			require.Equal(t, csvexport.StatusStorageFailed, pending.Status)
			bucket, err := blob.OpenBucket(ctx, repos.Attachments.GetConnString())
			require.NoError(t, err)
			defer bucket.Close()
			key := repos.Attachments.GetFullPath(row.BlobKey)
			exists, err := bucket.Exists(ctx, key)
			require.NoError(t, err)
			require.True(t, exists)
			fail = false
			now = now.Add(csvRetryDelay)
			fresh := NewCSVExportService(db, repos, nil)
			fresh.now = svc.now
			require.NoError(t, fresh.Reconcile(ctx))
			require.NoError(t, fresh.RunJob(ctx, row.ID)) // duplicate notification
			bytes, err := bucket.ReadAll(ctx, key)
			require.NoError(t, err)
			downloaded, err := fresh.Download(ctx, u.ID, nil, row.ID)
			require.NoError(t, err)
			require.Equal(t, bytes, downloaded)
			require.NotContains(t, string(bytes), "secret-hash")
			_, err = fresh.Status(ctx, other.ID, nil, row.ID)
			require.ErrorIs(t, err, ErrCSVAccess)
			_, err = fresh.Download(ctx, other.ID, nil, row.ID)
			require.ErrorIs(t, err, ErrCSVAccess)
			// Scoped missing deletion is deliberately non-revealing and idempotent.
			require.NoError(t, fresh.Delete(ctx, other.ID, nil, row.ID))
			protected, err := db.CSVExport.Get(ctx, row.ID)
			require.NoError(t, err)
			require.Equal(t, csvexport.StatusCompleted, protected.Status)
			history, err := fresh.History(ctx, other.ID, nil)
			require.NoError(t, err)
			require.Empty(t, history)
			complete, err := db.CSVExport.Get(ctx, row.ID)
			require.NoError(t, err)
			require.NoError(t, fresh.SetRetention(ctx, u.ID, nil, 1))
			for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
				now = complete.ExpiresAt.Add(offset)
				_, err = fresh.Download(ctx, u.ID, nil, row.ID)
				if offset < 0 {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, ErrCSVExpired)
				}
			}
			// Delayed cleanup does not change the fixed expiry or revive downloads.
			now = complete.ExpiresAt.Add(24 * time.Hour)
			fresh.remove = func(context.Context, string) error { return errors.New("injected blob deletion failure") }
			require.ErrorContains(t, fresh.Cleanup(ctx), "injected blob deletion failure")
			exists, err = bucket.Exists(ctx, key)
			require.NoError(t, err)
			require.True(t, exists)
			// Restart after failed deletion; then fail SQL after the real blob is
			// removed. Durable tombstones must survive both failure boundaries.
			fresh = NewCSVExportService(db, repos, nil)
			fresh.now = svc.now
			now = now.Add(csvRetryDelay)
			failDelete := true
			db.CSVExport.Use(func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
					if failDelete && m.Op().Is(ent.OpDeleteOne) {
						return nil, errors.New("injected metadata deletion failure")
					}
					return next.Mutate(ctx, m)
				})
			})
			require.ErrorContains(t, fresh.Cleanup(ctx), "injected metadata deletion failure")
			exists, err = bucket.Exists(ctx, key)
			require.NoError(t, err)
			require.False(t, exists)
			tombstone, err := db.CSVExport.Get(ctx, row.ID)
			require.NoError(t, err)
			require.Equal(t, csvexport.StatusCleanupPending, tombstone.Status)
			require.Equal(t, row.BlobKey, tombstone.BlobKey)
			failDelete = false
			now = now.Add(csvRetryDelay)
			require.NoError(t, fresh.Cleanup(ctx)) // missing blob is successful cleanup
			require.NoError(t, fresh.Cleanup(ctx))
			require.NoError(t, fresh.Delete(ctx, u.ID, nil, row.ID))
			require.NoError(t, fresh.RunJob(ctx, row.ID))
			exists, err = bucket.Exists(ctx, key)
			require.NoError(t, err)
			require.False(t, exists)
			_, err = db.CSVExport.Get(ctx, row.ID)
			require.True(t, ent.IsNotFound(err))

			// Membership revoked after enqueue blocks generation. Rejoining allows an
			// explicit retry, but a later revocation blocks every artifact operation.
			group, err := repos.Groups.GroupCreate(ctx, "collection", uuid.Nil)
			require.NoError(t, err)
			_, err = db.UserGroup.Create().SetUserID(u.ID).SetGroupID(group.ID).SetRole(usergroup.RoleOwner).Save(ctx)
			require.NoError(t, err)
			collectionCapture := CSVCaptureRequest{Source: "items", Scope: CSVCollection, CollectionID: &group.ID, Columns: []CSVColumn{"name"}}
			collectionRow, err := fresh.Enqueue(ctx, u.ID, collectionCapture)
			require.NoError(t, err)
			_, err = db.UserGroup.Delete().Where(usergroup.UserID(u.ID), usergroup.GroupID(group.ID)).Exec(ctx)
			require.NoError(t, err)
			require.Error(t, fresh.RunJob(ctx, collectionRow.ID))
			denied, err := db.CSVExport.Get(ctx, collectionRow.ID)
			require.NoError(t, err)
			require.Equal(t, csvexport.StatusGenerationFailed, denied.Status)
			exists, err = bucket.Exists(ctx, repos.Attachments.GetFullPath(collectionRow.BlobKey))
			require.NoError(t, err)
			require.False(t, exists)
			_, err = db.UserGroup.Create().SetUserID(u.ID).SetGroupID(group.ID).SetRole(usergroup.RoleOwner).Save(ctx)
			require.NoError(t, err)
			require.NoError(t, fresh.RetryArtifact(ctx, u.ID, &group.ID, collectionRow.ID))
			require.NoError(t, fresh.RunJob(ctx, collectionRow.ID))
			_, err = db.UserGroup.Delete().Where(usergroup.UserID(u.ID), usergroup.GroupID(group.ID)).Exec(ctx)
			require.NoError(t, err)
			_, err = fresh.Status(ctx, u.ID, &group.ID, collectionRow.ID)
			require.ErrorIs(t, err, ErrCSVAccess)
			_, err = fresh.History(ctx, u.ID, &group.ID)
			require.ErrorIs(t, err, ErrCSVAccess)
			_, err = fresh.Download(ctx, u.ID, &group.ID, collectionRow.ID)
			require.ErrorIs(t, err, ErrCSVAccess)
			require.ErrorIs(t, fresh.Delete(ctx, u.ID, &group.ID, collectionRow.ID), ErrCSVAccess)
			_, err = fresh.Retention(ctx, u.ID, &group.ID)
			require.ErrorIs(t, err, ErrCSVAccess)
			require.ErrorIs(t, fresh.SetRetention(ctx, u.ID, &group.ID, 2), ErrCSVAccess)
		})
	}
}
