package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/group"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/user"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"
)

var ErrCSVAccess = errors.New("CSV access denied")
var ErrCSVNotReady = errors.New("CSV not ready")
var ErrCSVExpired = errors.New("CSV expired")

// CSVStore makes write/close and delete failures testable. Put must not return
// success until the canonical object is committed; Open returns those bytes.
type CSVStore interface {
	Put(context.Context, string, []byte) error
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}
type csvBlobStore struct{ repos *repo.AllRepos }

func (s csvBlobStore) bucket(ctx context.Context) (*blob.Bucket, error) {
	return blob.OpenBucket(ctx, s.repos.Attachments.GetConnString())
}
func (s csvBlobStore) Put(ctx context.Context, key string, data []byte) error {
	b, e := s.bucket(ctx)
	if e != nil {
		return e
	}
	defer b.Close()
	return b.WriteAll(ctx, s.repos.Attachments.GetFullPath(key), data, &blob.WriterOptions{ContentType: "text/csv; charset=utf-8"})
}
func (s csvBlobStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	b, e := s.bucket(ctx)
	if e != nil {
		return nil, e
	}
	r, e := b.NewReader(ctx, s.repos.Attachments.GetFullPath(key), nil)
	if e != nil {
		b.Close()
		return nil, e
	}
	return &csvBlobReader{ReadCloser: r, bucket: b}, nil
}

type csvBlobReader struct {
	io.ReadCloser
	bucket *blob.Bucket
}

func (r *csvBlobReader) Close() error { e := r.ReadCloser.Close(); _ = r.bucket.Close(); return e }
func (s csvBlobStore) Delete(ctx context.Context, key string) error {
	b, e := s.bucket(ctx)
	if e != nil {
		return e
	}
	defer b.Close()
	e = b.Delete(ctx, s.repos.Attachments.GetFullPath(key))
	if gcerrors.Code(e) == gcerrors.NotFound {
		return nil
	}
	return e
}

type CSVExportService struct {
	repos *repo.AllRepos
	store CSVStore
	now   func() time.Time
}

func (s *CSVExportService) clock() time.Time {
	if s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
func (s *CSVExportService) key(row *ent.CsvExport) string {
	if row.GroupID != nil {
		return fmt.Sprintf("%s/csv/%s.csv", *row.GroupID, row.ID)
	}
	return fmt.Sprintf("users/%s/csv/%s.csv", row.RequesterID, row.ID)
}
func (s *CSVExportService) Create(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, req csvexports.Request) (csvexports.Artifact, error) {
	if (req.Scope == "collection") != (gid != nil) {
		return csvexports.Artifact{}, ErrCSVAccess
	}
	if _, e := s.repos.CsvExports.ResolveSource(ctx, uid, gid, req); e != nil {
		return csvexports.Artifact{}, ErrCSVAccess
	}
	row, e := s.repos.CsvExports.Create(ctx, uid, gid, req)
	if e != nil {
		return csvexports.Artifact{}, e
	}
	return repo.CsvExportMetadata(row), nil
}
func (s *CSVExportService) get(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) (*ent.CsvExport, error) {
	q := s.repos.CsvExports.PrivateQuery(uid)
	if gid != nil {
		q = s.repos.CsvExports.CollectionQuery(*gid)
	}
	row, e := q.Where(csvexport.ID(id)).Only(ctx)
	if e != nil {
		return nil, e
	}
	if e = s.repos.CsvExports.AuthorizeArtifact(ctx, uid, gid, row); e != nil {
		return nil, ErrCSVAccess
	}
	return row, nil
}
func (s *CSVExportService) Get(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) (csvexports.Artifact, error) {
	row, e := s.get(ctx, uid, gid, id)
	if e != nil {
		return csvexports.Artifact{}, e
	}
	return repo.CsvExportMetadata(row), nil
}
func (s *CSVExportService) List(ctx context.Context, uid uuid.UUID, gid *uuid.UUID) ([]csvexports.Artifact, error) {
	// Validate identity/membership even when history is empty.
	if gid != nil {
		ok, e := s.repos.CsvExports.LifecycleClient().Group.Query().Where(group.ID(*gid), group.HasUsersWith(user.ID(uid))).Exist(ctx)
		if e != nil {
			return nil, e
		}
		if !ok {
			return nil, ErrCSVAccess
		}
	}
	q := s.repos.CsvExports.PrivateQuery(uid)
	if gid != nil {
		q = s.repos.CsvExports.CollectionQuery(*gid)
	}
	rows, e := q.Order(ent.Desc(csvexport.FieldCreatedAt)).All(ctx)
	if e != nil {
		return nil, e
	}
	out := []csvexports.Artifact{}
	for _, row := range rows {
		if s.repos.CsvExports.AuthorizeArtifact(ctx, uid, gid, row) == nil {
			out = append(out, repo.CsvExportMetadata(row))
		}
	}
	return out, nil
}
func (s *CSVExportService) Download(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) (io.ReadCloser, csvexports.Artifact, error) {
	row, e := s.get(ctx, uid, gid, id)
	if e != nil {
		return nil, csvexports.Artifact{}, e
	}
	if row.ExpiresAt != nil && !s.clock().Before(*row.ExpiresAt) {
		return nil, csvexports.Artifact{}, ErrCSVExpired
	}
	if row.Status != csvexport.StatusCompleted || row.ArtifactPath != s.key(row) {
		return nil, csvexports.Artifact{}, ErrCSVNotReady
	}
	r, e := s.store.Open(ctx, row.ArtifactPath)
	if e != nil {
		return nil, csvexports.Artifact{}, e
	}
	if row.ExpiresAt == nil || !s.clock().Before(*row.ExpiresAt) {
		_ = r.Close()
		return nil, csvexports.Artifact{}, ErrCSVExpired
	}
	return r, repo.CsvExportMetadata(row), nil
}
func (s *CSVExportService) Retry(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) error {
	row, e := s.get(ctx, uid, gid, id)
	if e != nil {
		return e
	}
	if row.RequesterID != uid {
		return ErrCSVAccess
	}
	n, e := s.repos.CsvExports.LifecycleClient().CsvExport.Update().Where(csvexport.ID(id), csvexport.StatusEQ(csvexport.StatusFailed)).SetStatus(csvexport.StatusPending).ClearError().Save(ctx)
	if e == nil && n == 0 {
		return ErrCSVNotReady
	}
	return e
}
func (s *CSVExportService) Delete(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) error {
	row, e := s.get(ctx, uid, gid, id)
	if ent.IsNotFound(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if row.RequesterID != uid {
		if gid == nil || s.repos.CsvExports.IsOwner(ctx, uid, *gid) != nil {
			return ErrCSVAccess
		}
	}
	return s.remove(ctx, row)
}
func (s *CSVExportService) remove(ctx context.Context, row *ent.CsvExport) error {
	db := s.repos.CsvExports.LifecycleClient()
	n, e := db.CsvExport.Update().Where(csvexport.ID(row.ID), csvexport.StatusNEQ(csvexport.StatusRunning)).SetStatus(csvexport.StatusDeleting).Save(ctx)
	if e != nil {
		return e
	}
	if n == 0 {
		// A concurrent delete may already have removed it.
		if exists, err := db.CsvExport.Query().Where(csvexport.ID(row.ID)).Exist(ctx); err == nil && !exists {
			return nil
		}
		return ErrCSVNotReady
	}
	// The caller's snapshot can predate a completed/failed write. Reload the
	// pointer after the deleting claim; no writer can now acquire this row.
	row, e = db.CsvExport.Get(ctx, row.ID)
	if ent.IsNotFound(e) {
		return nil
	}
	if e != nil {
		return e
	}
	// Pointer is persisted BEFORE any write and retained until deletion succeeds.
	if row.ArtifactPath != "" {
		if row.ArtifactPath != s.key(row) {
			return ErrCSVAccess
		}
		if e = s.store.Delete(ctx, row.ArtifactPath); e != nil {
			return e
		}
	}
	_, e = db.CsvExport.Delete().Where(csvexport.ID(row.ID), csvexport.StatusEQ(csvexport.StatusDeleting)).Exec(ctx)
	return e
}

// Sweep is a separate durable CSV worker. Database claims, not memory pubsub,
// arbitrate jobs. Jobs have a five-minute deadline; a ten-minute abandoned claim
// can be recovered after a crash. Failed jobs require an explicit authorized retry.
func (s *CSVExportService) Sweep(ctx context.Context) error {
	db := s.repos.CsvExports.LifecycleClient()
	_, e := db.CsvExport.Update().Where(csvexport.StatusEQ(csvexport.StatusRunning), csvexport.UpdatedAtLT(s.clock().Add(-10*time.Minute))).SetStatus(csvexport.StatusPending).Save(ctx)
	if e != nil {
		return e
	}
	rows, e := db.CsvExport.Query().All(ctx)
	if e != nil {
		return e
	}
	var errs []error
	for _, row := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if row.Status == csvexport.StatusDeleting || row.ExpiresAt != nil && !s.clock().Before(*row.ExpiresAt) {
			if e := s.remove(ctx, row); e != nil {
				errs = append(errs, e)
			}
			continue
		}
		if row.Status == csvexport.StatusPending {
			if e := s.run(ctx, row); e != nil {
				errs = append(errs, e)
			}
		}
	}
	return errors.Join(errs...)
}
func (s *CSVExportService) run(parent context.Context, row *ent.CsvExport) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	db := s.repos.CsvExports.LifecycleClient()
	// The claim timestamp fences a late failure/completion from a superseded
	// worker; checking status alone could mutate a newer running attempt.
	claim := s.clock()
	n, e := db.CsvExport.Update().Where(csvexport.ID(row.ID), csvexport.StatusEQ(csvexport.StatusPending)).SetUpdatedAt(claim).SetStatus(csvexport.StatusRunning).SetArtifactPath(s.key(row)).Save(ctx)
	if e != nil || n == 0 {
		return e
	}
	fail := func(e error) error { // No driver/storage details in public diagnostics.
		_, saveErr := db.CsvExport.Update().Where(csvexport.ID(row.ID), csvexport.StatusEQ(csvexport.StatusRunning), csvexport.UpdatedAtEQ(claim)).SetStatus(csvexport.StatusFailed).SetError("CSV generation or storage failed; retry available").Save(parent)
		return errors.Join(e, saveErr)
	}
	rows, e := s.repos.CsvExports.ResolveSource(ctx, row.RequesterID, row.GroupID, row.RequestSnapshot)
	if e != nil {
		return fail(e)
	}
	var data bytes.Buffer
	if e = csvexports.Write(&data, row.RequestSnapshot, rows); e != nil {
		return fail(e)
	}
	if e = s.store.Put(ctx, s.key(row), data.Bytes()); e != nil {
		return fail(e)
	}
	// Recheck after storage, so a permission change during processing cannot expose
	// the artifact as completed. Failed artifacts are never downloadable.
	if e = s.repos.CsvExports.AuthorizeArtifact(ctx, row.RequesterID, row.GroupID, row); e != nil {
		return fail(e)
	}
	completed := s.clock()
	expires := completed.Add(time.Duration(row.RetentionDays) * 24 * time.Hour)
	n, e = db.CsvExport.Update().Where(csvexport.ID(row.ID), csvexport.StatusEQ(csvexport.StatusRunning), csvexport.UpdatedAtEQ(claim)).SetStatus(csvexport.StatusCompleted).SetCompletedAt(completed).SetExpiresAt(expires).SetSizeBytes(int64(data.Len())).ClearError().Save(ctx)
	if e != nil {
		return fail(e)
	}
	if n == 0 {
		return ErrCSVNotReady
	}
	return nil
}

// PurgeIdentity is called before account/collection deletion. Restrictive FKs
// remain a last line of defence against concurrent creates and orphaned objects.
func (s *CSVExportService) PurgeIdentity(ctx context.Context, uid, gid *uuid.UUID) error {
	q := s.repos.CsvExports.LifecycleClient().CsvExport.Query()
	if uid != nil {
		q.Where(csvexport.RequesterID(*uid))
	}
	if gid != nil {
		q.Where(csvexport.GroupID(*gid))
	}
	rows, e := q.All(ctx)
	if e != nil {
		return e
	}
	for _, row := range rows {
		if e = s.remove(ctx, row); e != nil {
			return e
		}
	}
	return nil
}
