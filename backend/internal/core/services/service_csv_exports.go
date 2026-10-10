package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"gocloud.dev/blob"
)

const csvLease = 15 * time.Minute
const csvRetryDelay = time.Minute

type csvRowSource interface {
	Load(context.Context, uuid.UUID, CSVCaptureRequest) (CSVInventoryRows, error)
}

// CSVExportService uses SQL as its durable queue, not process-local pubsub.
// The immutable blob key is allocated before upload and retained on every error.
// Publication holds a database write lock through upload and completion, fencing
// expired leases: two deliveries can generate but cannot publish concurrently.
// On SQLite this serializes database writes during upload; see csv_generation.md.
type CSVExportService struct {
	db      *ent.Client
	repos   *repo.AllRepos
	sources csvRowSource
	now     func() time.Time
	upload  func(context.Context, string, io.Reader) error
}

func NewCSVExportService(db *ent.Client, repos *repo.AllRepos, boundaries CSVBoundaryAuthority) *CSVExportService {
	return &CSVExportService{db: db, repos: repos, sources: NewCSVSources(db, boundaries), now: time.Now,
		upload: func(ctx context.Context, key string, src io.Reader) error {
			bucket, err := blob.OpenBucket(ctx, repos.Attachments.GetConnString())
			if err != nil {
				return err
			}
			defer func() { _ = bucket.Close() }()
			w, err := bucket.NewWriter(ctx, repos.Attachments.GetFullPath(key), &blob.WriterOptions{ContentType: "text/csv; charset=utf-8"})
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(w, src)
			closeErr := w.Close()
			return errors.Join(copyErr, closeErr)
		},
	}
}

// Enqueue persists the complete authorized capture before returning. The polling
// worker requires no in-memory notification, so submission survives restart.
func (s *CSVExportService) Enqueue(ctx context.Context, uid uuid.UUID, r CSVCaptureRequest) (*ent.CSVExport, error) {
	if _, err := s.sources.Load(ctx, uid, r); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	// Apply the same bounded capture contract to direct service callers as API
	// callers, so no persisted job is too large to decode during generation.
	if _, err := DecodeCSVCapture(bytes.NewReader(raw)); err != nil {
		return nil, err
	}
	var capture map[string]interface{}
	if err := json.Unmarshal(raw, &capture); err != nil {
		return nil, err
	}
	return s.repos.CSVExports.Create(ctx, uid, r.CollectionID, csvexport.Scope(r.Scope), string(r.Source), capture)
}

func csvJobCapture(row *ent.CSVExport) (CSVCaptureRequest, error) {
	var r CSVCaptureRequest
	b, err := json.Marshal(row.Capture)
	if err != nil {
		return r, err
	}
	r, err = DecodeCSVCapture(bytes.NewReader(b))
	if err != nil {
		return r, err
	}
	if string(r.Source) != row.Source || string(r.Scope) != string(row.Scope) || (r.CollectionID == nil) != (row.GroupID == nil) {
		return r, fmt.Errorf("csv: inconsistent persisted capture")
	}
	if r.CollectionID != nil && *r.CollectionID != *row.GroupID {
		return r, errCSVAccess
	}
	return r, nil
}

// Retry is explicit for generation failures (including revoked access / invalid
// source). Storage failures retry automatically. Callers must still authorize
// artifact access; this primitive restricts retry to the original requester.
func (s *CSVExportService) Retry(ctx context.Context, uid, id uuid.UUID) error {
	row, err := s.db.CSVExport.Get(ctx, id)
	if err != nil {
		return err
	}
	if row.UserID != uid {
		return errCSVAccess
	}
	r, err := csvJobCapture(row)
	if err != nil {
		return err
	}
	if _, err := s.sources.Load(ctx, uid, r); err != nil {
		return err
	}
	_, err = s.db.CSVExport.Update().Where(csvexport.IDEQ(id), csvexport.StatusIn(csvexport.StatusGenerationFailed, csvexport.StatusStorageFailed)).SetStatus(csvexport.StatusQueued).ClearRetryAt().ClearFailureCode().ClearFailureDetail().Save(ctx)
	return err
}

func csvReady() []csvexport.Status {
	return []csvexport.Status{csvexport.StatusQueued, csvexport.StatusRunning, csvexport.StatusStorageFailed}
}

// RunJob is idempotent on completed/active deliveries. Running jobs become
// eligible after their persisted lease expires. Attempts are fencing tokens.
func (s *CSVExportService) RunJob(ctx context.Context, id uuid.UUID) error {
	now := s.now().UTC()
	row, err := s.db.CSVExport.Get(ctx, id)
	if err != nil {
		return err
	}
	n, err := s.db.CSVExport.Update().Where(csvexport.IDEQ(id), csvexport.AttemptsEQ(row.Attempts), csvexport.StatusIn(csvReady()...),
		csvexport.Or(csvexport.RetryAtIsNil(), csvexport.RetryAtLTE(now))).
		SetStatus(csvexport.StatusRunning).AddAttempts(1).SetRetryAt(now.Add(csvLease)).ClearFailureCode().ClearFailureDetail().Save(ctx)
	if err != nil || n == 0 {
		return err
	}
	row.Attempts++
	r, err := csvJobCapture(row)
	var data CSVInventoryRows
	if err == nil {
		data, err = s.sources.Load(ctx, row.UserID, r)
	}
	if err != nil {
		return s.fail(ctx, row, csvexport.StatusGenerationFailed, "source", err)
	}
	// Spool locally before opening a blob writer. Encoding failures never publish
	// a truncated artifact. Row adapters load a complete snapshot without a cap.
	file, err := os.CreateTemp("", "homebox-csv-*")
	if err != nil {
		return s.fail(ctx, row, csvexport.StatusStorageFailed, "spool", err)
	}
	defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
	if err := WriteCanonicalCSV(file, data); err != nil {
		if errors.Is(err, errCSVEncoding) {
			return s.fail(ctx, row, csvexport.StatusGenerationFailed, "encoding", err)
		}
		return s.fail(ctx, row, csvexport.StatusStorageFailed, "spool", err)
	}
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return s.fail(ctx, row, csvexport.StatusStorageFailed, "spool", err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return s.fail(ctx, row, csvexport.StatusStorageFailed, "spool", err)
	}

	tx, err := s.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// UPDATE locks the row in PostgreSQL and the writer in SQLite. A lease
	// takeover cannot pass this lock until upload+completion has committed.
	n, err = tx.CSVExport.Update().Where(csvexport.IDEQ(id), csvexport.StatusEQ(csvexport.StatusRunning), csvexport.AttemptsEQ(row.Attempts)).SetRetryAt(s.now().Add(csvLease)).Save(ctx)
	if err != nil || n == 0 {
		return err
	}
	if err := s.upload(ctx, row.BlobKey, file); err != nil {
		_ = tx.Rollback()
		return s.fail(ctx, row, csvexport.StatusStorageFailed, "upload", err)
	}
	completed := s.now().UTC()
	var days int
	if row.GroupID != nil {
		g, e := tx.Group.Get(ctx, *row.GroupID)
		err = e
		if e == nil {
			days = g.CsvRetentionDays
		}
	} else {
		u, e := tx.User.Get(ctx, row.UserID)
		err = e
		if e == nil {
			days = u.CsvRetentionDays
		}
	}
	if err == nil {
		err = repo.ValidateCSVRetentionDays(days)
	}
	if err == nil {
		_, err = tx.CSVExport.UpdateOneID(id).SetStatus(csvexport.StatusCompleted).SetCompletedAt(completed).
			SetExpiresAt(completed.Add(time.Duration(days) * 24 * time.Hour)).SetSizeBytes(size).ClearRetryAt().ClearFailureCode().ClearFailureDetail().Save(ctx)
	}
	if err != nil {
		_ = tx.Rollback()
		return s.fail(ctx, row, csvexport.StatusStorageFailed, "metadata", err)
	}
	// An ambiguous commit is never followed by destructive blob deletion. The
	// durable row either reads completed or stays running for reconciliation.
	if err := tx.Commit(); err != nil {
		// The conditional error write cannot regress a committed completion.
		// If SQL is still unavailable, the running lease remains recoverable.
		return s.fail(ctx, row, csvexport.StatusStorageFailed, "metadata", err)
	}
	return nil
}

func (s *CSVExportService) fail(ctx context.Context, row *ent.CSVExport, status csvexport.Status, code string, cause error) error {
	update := s.db.CSVExport.Update().Where(csvexport.IDEQ(row.ID), csvexport.StatusEQ(csvexport.StatusRunning), csvexport.AttemptsEQ(row.Attempts)).SetStatus(status).SetFailureCode(code).ClearFailureDetail()
	if status == csvexport.StatusStorageFailed {
		update.SetRetryAt(s.now().Add(csvRetryDelay))
	} else {
		update.ClearRetryAt()
	}
	_, err := update.Save(ctx)
	return errors.Join(cause, err)
}

// Reconcile recovers queued jobs, expired running leases and due storage retries.
// No job is dropped on process shutdown or transient SQL/blob failure.
func (s *CSVExportService) Reconcile(ctx context.Context) error {
	now := s.now()
	rows, err := s.db.CSVExport.Query().Where(csvexport.StatusIn(csvReady()...), csvexport.Or(csvexport.RetryAtIsNil(), csvexport.RetryAtLTE(now))).Order(ent.Asc(csvexport.FieldCreatedAt)).All(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, row := range rows {
		if ctx.Err() != nil {
			return errors.Join(append(errs, ctx.Err())...)
		}
		if err := s.RunJob(ctx, row.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
