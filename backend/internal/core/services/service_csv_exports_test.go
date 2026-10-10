package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
	"gocloud.dev/blob"
)

type csvFakeSource struct {
	mu    sync.Mutex
	data  CSVInventoryRows
	err   error
	calls int
}

func (f *csvFakeSource) Load(context.Context, uuid.UUID, CSVCaptureRequest) (CSVInventoryRows, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.data, f.err
}

func csvWorkerFixture(t *testing.T) (*CSVExportService, *csvFakeSource, *ent.CSVExport, *time.Time, *map[string][]byte) {
	t.Helper()
	ctx := context.Background()
	db, err := ent.Open("sqlite3", "file:"+uuid.NewString()+"?mode=memory&cache=shared&_fk=1&_time_format=sqlite")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Schema.Create(ctx))
	_, err = db.User.Create().SetID(tUser.ID).SetName("worker").SetEmail("worker@example.invalid").Save(ctx)
	require.NoError(t, err)
	repos := repo.New(db, nil, config.Storage{}, "", config.Thumbnail{})
	svc := NewCSVExportService(db, repos, nil)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	source := &csvFakeSource{data: CSVInventoryRows{Columns: []CSVColumn{"name"}, Rows: [][]any{{"雪,\"quoted\"\nline"}, {"\t=1"}}}}
	svc.sources = source
	objects := map[string][]byte{}
	svc.upload = func(_ context.Context, key string, r io.Reader) error {
		b, err := io.ReadAll(r)
		objects[key] = b
		return err
	}
	r := CSVCaptureRequest{Source: "account-api-keys", Scope: CSVAccountPrivate, Columns: []CSVColumn{"name"}}
	row, err := svc.Enqueue(ctx, tUser.ID, r)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.db.CSVExport.DeleteOneID(row.ID).Exec(ctx) })
	return svc, source, row, &now, &objects
}

func TestCSVWorkerCanonicalCompletionAndDuplicate(t *testing.T) {
	ctx := context.Background()
	svc, src, row, now, objects := csvWorkerFixture(t)
	// More rows than ordinary list page sizes; worker imposes no extra cap.
	for i := 0; i < 1001; i++ {
		src.data.Rows = append(src.data.Rows, []any{"row"})
	}
	require.NoError(t, svc.RunJob(ctx, row.ID))
	got, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusCompleted, got.Status)
	require.True(t, now.Equal(*got.CompletedAt))
	require.True(t, now.Add(30*24*time.Hour).Equal(*got.ExpiresAt))
	var expected bytes.Buffer
	require.NoError(t, WriteCanonicalCSV(&expected, src.data))
	require.Equal(t, expected.Bytes(), (*objects)[row.BlobKey])
	require.Equal(t, int64(expected.Len()), got.SizeBytes)
	calls := src.calls
	require.NoError(t, svc.RunJob(ctx, row.ID))
	require.Equal(t, calls, src.calls)
	require.Len(t, *objects, 1)
}

func TestCSVWorkerFailuresAndRestart(t *testing.T) {
	ctx := context.Background()
	svc, src, row, now, objects := csvWorkerFixture(t)
	src.err = errors.New("source revoked")
	require.ErrorContains(t, svc.RunJob(ctx, row.ID), "source revoked")
	got, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusGenerationFailed, got.Status)
	require.Nil(t, got.CompletedAt)
	require.Empty(t, *objects)
	src.err = nil
	require.NoError(t, svc.Retry(ctx, tUser.ID, row.ID))
	upload := svc.upload
	svc.upload = func(_ context.Context, key string, _ io.Reader) error {
		(*objects)[key] = []byte("partial")
		return errors.New("upload interrupted")
	}
	require.ErrorContains(t, svc.RunJob(ctx, row.ID), "upload interrupted")
	got, err = svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusStorageFailed, got.Status)
	require.Nil(t, got.CompletedAt)
	require.Equal(t, row.BlobKey, got.BlobKey)
	// Simulated fresh process has no notification or memory queue, just SQL.
	fresh := NewCSVExportService(svc.db, svc.repos, nil)
	fresh.now, fresh.sources, fresh.upload = svc.now, src, upload
	*now = now.Add(csvRetryDelay)
	require.NoError(t, fresh.Reconcile(ctx))
	got, err = svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusCompleted, got.Status)
	require.NotEqual(t, []byte("partial"), (*objects)[row.BlobKey])
	require.Len(t, *objects, 1)
}

func TestCSVWorkerExpiredLeaseAndStaleDelivery(t *testing.T) {
	ctx := context.Background()
	svc, src, row, now, _ := csvWorkerFixture(t)
	require.NoError(t, svc.db.CSVExport.UpdateOneID(row.ID).SetStatus(csvexport.StatusRunning).SetAttempts(1).SetRetryAt(now.Add(csvLease)).Exec(ctx))
	calls := src.calls
	require.NoError(t, svc.RunJob(ctx, row.ID))
	require.Equal(t, calls, src.calls)
	*now = now.Add(csvLease)
	require.NoError(t, svc.RunJob(ctx, row.ID))
	got, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 2, got.Attempts)
	require.Equal(t, csvexport.StatusCompleted, got.Status)
	// A stale delivery cannot regress completed metadata on failure.
	row.Attempts = 1
	require.Error(t, svc.fail(ctx, row, csvexport.StatusStorageFailed, "upload", errors.New("stale")))
	got, err = svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusCompleted, got.Status)
}

func TestCSVWorkerSQLFailureAfterUpload(t *testing.T) {
	ctx := context.Background()
	svc, _, row, now, objects := csvWorkerFixture(t)
	// Fail exactly the completion mutation, after upload, without breaking the
	// failure-state write. The blob stays associated with durable retry intent.
	fail := true
	svc.db.CSVExport.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			cm, ok := m.(*ent.CSVExportMutation)
			if ok {
				status, set := cm.Status()
				id, _ := cm.ID()
				if set && status == csvexport.StatusCompleted && id == row.ID && fail {
					return nil, errors.New("SQL completion unavailable")
				}
			}
			return next.Mutate(ctx, m)
		})
	})
	require.ErrorContains(t, svc.RunJob(ctx, row.ID), "SQL completion unavailable")
	got, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusStorageFailed, got.Status)
	require.Equal(t, "metadata", got.FailureCode)
	require.Nil(t, got.CompletedAt)
	require.Contains(t, *objects, got.BlobKey)
	fail = false
	*now = now.Add(csvRetryDelay)
	require.NoError(t, svc.RunJob(ctx, row.ID))
	got, err = svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusCompleted, got.Status)
	require.Len(t, *objects, 1)
}

type csvSourceFunc func(context.Context, uuid.UUID, CSVCaptureRequest) (CSVInventoryRows, error)

func (f csvSourceFunc) Load(ctx context.Context, uid uuid.UUID, r CSVCaptureRequest) (CSVInventoryRows, error) {
	return f(ctx, uid, r)
}

func TestCSVWorkerFencesExpiredGenerator(t *testing.T) {
	ctx := context.Background()
	svc, src, row, now, objects := csvWorkerFixture(t)
	old := true
	uploads := 0
	upload := svc.upload
	svc.upload = func(ctx context.Context, key string, r io.Reader) error { uploads++; return upload(ctx, key, r) }
	svc.sources = csvSourceFunc(func(ctx context.Context, uid uuid.UUID, r CSVCaptureRequest) (CSVInventoryRows, error) {
		if old {
			old = false
			*now = now.Add(csvLease)
			// A newer delivery claims/completes while the first is still generating.
			require.NoError(t, svc.RunJob(ctx, row.ID))
			return CSVInventoryRows{Columns: []CSVColumn{"name"}, Rows: [][]any{{"STALE"}}}, nil
		}
		return src.data, nil
	})
	require.NoError(t, svc.RunJob(ctx, row.ID))
	require.Equal(t, 1, uploads)
	require.NotContains(t, string((*objects)[row.BlobKey]), "STALE")
	got, err := svc.db.CSVExport.Get(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 2, got.Attempts)
	require.Equal(t, csvexport.StatusCompleted, got.Status)
}

func TestCSVWorkerActualAdapterAndBlob(t *testing.T) {
	ctx := context.Background()
	svc, _, row, _, _ := csvWorkerFixture(t)
	repos := repo.New(svc.db, nil, config.Storage{ConnString: "file://" + t.TempDir(), PrefixPath: "artifacts"}, "", config.Thumbnail{})
	real := NewCSVExportService(svc.db, repos, nil)
	real.now = svc.now
	_, err := repos.APIKeys.Create(ctx, tUser.ID, "\t=Unicode雪,\"quoted\"\nnext", []byte("already-hashed-not-exported"), nil)
	require.NoError(t, err)
	r, err := csvJobCapture(row)
	require.NoError(t, err)
	data, err := real.sources.Load(ctx, tUser.ID, r)
	require.NoError(t, err)
	var expected bytes.Buffer
	require.NoError(t, WriteCanonicalCSV(&expected, data))
	require.NoError(t, real.RunJob(ctx, row.ID))
	bucket, err := blob.OpenBucket(ctx, repos.Attachments.GetConnString())
	require.NoError(t, err)
	defer func() { _ = bucket.Close() }()
	localSave, err := bucket.ReadAll(ctx, repos.Attachments.GetFullPath(row.BlobKey))
	require.NoError(t, err)
	retainedDownload, err := bucket.ReadAll(ctx, repos.Attachments.GetFullPath(row.BlobKey))
	require.NoError(t, err)
	require.Equal(t, expected.Bytes(), localSave)
	require.Equal(t, localSave, retainedDownload)
	require.NotContains(t, string(localSave), "already-hashed")
	require.NoError(t, real.RunJob(ctx, row.ID))
	after, err := bucket.ReadAll(ctx, repos.Attachments.GetFullPath(row.BlobKey))
	require.NoError(t, err)
	require.Equal(t, localSave, after)
}
