package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"gocloud.dev/blob"
)

var (
	ErrCSVAccess   = errors.New("CSV artifact unavailable")
	ErrCSVExpired  = errors.New("CSV artifact expired")
	ErrCSVNotReady = errors.New("CSV artifact not ready")
	ErrCSVStorage  = errors.New("CSV artifact storage unavailable")
)

// CSVArtifact is the public lifecycle projection. Internal capture, storage keys
// and diagnostic details must never be serialized by a lifecycle endpoint.
type CSVArtifact struct {
	ID          uuid.UUID        `json:"id"`
	Source      string           `json:"source"`
	Scope       csvexport.Scope  `json:"scope"`
	Status      csvexport.Status `json:"status"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	CompletedAt *time.Time       `json:"completedAt"`
	ExpiresAt   *time.Time       `json:"expiresAt"`
	SizeBytes   int64            `json:"sizeBytes"`
	FailureCode string           `json:"failureCode,omitempty"`
	RetryMode   string           `json:"retryMode"`
	RetryAt     *time.Time       `json:"retryAt,omitempty"`
}

func CSVArtifactView(row *ent.CSVExport) CSVArtifact {
	mode := "none"
	if row.Status == csvexport.StatusGenerationFailed {
		mode = "requester-explicit"
	}
	if row.Status == csvexport.StatusStorageFailed {
		mode = "automatic-or-requester-explicit"
	}
	return CSVArtifact{row.ID, row.Source, row.Scope, row.Status, row.CreatedAt, row.UpdatedAt, row.CompletedAt, row.ExpiresAt, row.SizeBytes, row.FailureCode, mode, row.RetryAt}
}

func (s *CSVExportService) lifecycleScope(ctx context.Context, uid uuid.UUID, gid *uuid.UUID) error {
	if uid == uuid.Nil {
		return ErrCSVAccess
	}
	if _, err := s.repos.Users.GetOneID(ctx, uid); err != nil {
		return ErrCSVAccess
	}
	if gid != nil {
		member, err := s.repos.Groups.IsMember(ctx, *gid, uid)
		if err != nil {
			return err
		}
		if !member {
			return ErrCSVAccess
		}
	}
	return nil
}

func (s *CSVExportService) authorizeArtifact(ctx context.Context, uid uuid.UUID, row *ent.CSVExport) (CSVInventoryRows, error) {
	r, err := csvJobCapture(row)
	if err != nil {
		return CSVInventoryRows{}, ErrCSVAccess
	}
	spec, ok := CSVSourceDefinition(r.Source)
	if !ok || ((spec.Access == CSVUser || spec.Access == CSVMemberUser || row.GroupID == nil) && row.UserID != uid) {
		return CSVInventoryRows{}, ErrCSVAccess
	}
	// Even a persisted key cannot be used to access a different blob namespace.
	if row.BlobKey != fmt.Sprintf("csv/%s/%s.csv", row.UserID, row.ID) {
		return CSVInventoryRows{}, ErrCSVAccess
	}
	data, err := s.sources.Load(ctx, uid, r)
	if err != nil {
		return CSVInventoryRows{}, ErrCSVAccess
	}
	return data, nil
}

func (s *CSVExportService) artifact(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) (*ent.CSVExport, error) {
	if err := s.lifecycleScope(ctx, uid, gid); err != nil {
		return nil, err
	}
	var row *ent.CSVExport
	var err error
	if gid == nil {
		row, err = s.repos.CSVExports.GetPrivate(ctx, uid, id)
	} else {
		row, err = s.repos.CSVExports.GetByGroup(ctx, *gid, id)
	}
	if ent.IsNotFound(err) {
		return nil, ErrCSVAccess
	}
	if err != nil {
		return nil, err
	}
	if _, err = s.authorizeArtifact(ctx, uid, row); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *CSVExportService) Status(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) (CSVArtifact, error) {
	row, err := s.artifact(ctx, uid, gid, id)
	if err != nil {
		return CSVArtifact{}, err
	}
	return CSVArtifactView(row), nil
}

func (s *CSVExportService) History(ctx context.Context, uid uuid.UUID, gid *uuid.UUID) ([]CSVArtifact, error) {
	if err := s.lifecycleScope(ctx, uid, gid); err != nil {
		return nil, err
	}
	var rows []*ent.CSVExport
	var err error
	if gid == nil {
		rows, err = s.repos.CSVExports.ListPrivate(ctx, uid)
	} else {
		rows, err = s.repos.CSVExports.ListByGroup(ctx, *gid)
	}
	if err != nil {
		return nil, err
	}
	out := make([]CSVArtifact, 0, len(rows))
	for _, row := range rows {
		if _, err := s.authorizeArtifact(ctx, uid, row); err == nil {
			out = append(out, CSVArtifactView(row))
		}
	}
	return out, nil
}

// Download fails closed if current canonical source bytes differ from the
// retained snapshot. This conservative rule detects removed records, changed
// contexts and restricted rows even when the source's membership check passes.
// It can also deny benign edits; clients must request a fresh export then.
// No blob bytes are served until the entire comparison and expiry check pass.
func (s *CSVExportService) Download(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) ([]byte, error) {
	row, err := s.artifact(ctx, uid, gid, id)
	if err != nil {
		return nil, err
	}
	if row.Status != csvexport.StatusCompleted {
		return nil, ErrCSVNotReady
	}
	if row.ExpiresAt == nil || !s.now().Before(*row.ExpiresAt) {
		return nil, ErrCSVExpired
	}
	data, err := s.authorizeArtifact(ctx, uid, row)
	if err != nil {
		return nil, err
	}
	var current bytes.Buffer
	if err := WriteCanonicalCSV(&current, data); err != nil {
		return nil, ErrCSVAccess
	}
	retained, err := s.readArtifact(ctx, row.BlobKey)
	if err != nil {
		return nil, ErrCSVStorage
	}
	if !bytes.Equal(current.Bytes(), retained) {
		return nil, ErrCSVAccess
	}
	// Recheck after potentially slow storage/source operations, and prevent a
	// completed delivery that raced a deletion from becoming downloadable.
	latest, err := s.artifact(ctx, uid, gid, id)
	if err != nil {
		return nil, err
	}
	if latest.Status != csvexport.StatusCompleted {
		return nil, ErrCSVNotReady
	}
	data, err = s.authorizeArtifact(ctx, uid, latest)
	if err != nil {
		return nil, err
	}
	current.Reset()
	if err := WriteCanonicalCSV(&current, data); err != nil || !bytes.Equal(current.Bytes(), retained) {
		return nil, ErrCSVAccess
	}
	if latest.ExpiresAt == nil || !s.now().Before(*latest.ExpiresAt) {
		return nil, ErrCSVExpired
	}
	return retained, nil
}

func (s *CSVExportService) readArtifact(ctx context.Context, key string) ([]byte, error) {
	if s.read != nil {
		return s.read(ctx, key)
	}
	bucket, err := blob.OpenBucket(ctx, s.repos.Attachments.GetConnString())
	if err != nil {
		return nil, err
	}
	defer func() { _ = bucket.Close() }()
	reader, err := bucket.NewReader(ctx, s.repos.Attachments.GetFullPath(key), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
}

// Delete records a durable tombstone before any cleanup. Repeated deletion is
// safe, and the worker cannot publish after this transition. Blob-first cleanup
// of these intents is performed by the separate retention/cleanup capability.
func (s *CSVExportService) Delete(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) error {
	if err := s.lifecycleScope(ctx, uid, gid); err != nil {
		return err
	}
	var row *ent.CSVExport
	var err error
	if gid == nil {
		row, err = s.repos.CSVExports.GetPrivate(ctx, uid, id)
	} else {
		row, err = s.repos.CSVExports.GetByGroup(ctx, *gid, id)
	}
	// A missing scoped row is already deleted; this also avoids disclosing
	// whether an ID belongs to some other user or collection.
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := s.authorizeArtifact(ctx, uid, row); err != nil {
		return err
	}
	if row.Status == csvexport.StatusCleanupPending {
		return nil
	}
	return s.db.CSVExport.UpdateOneID(id).SetStatus(csvexport.StatusCleanupPending).ClearRetryAt().ClearFailureCode().ClearFailureDetail().Exec(ctx)
}

func (s *CSVExportService) RetryArtifact(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, id uuid.UUID) error {
	row, err := s.artifact(ctx, uid, gid, id)
	if err != nil {
		return err
	}
	if row.UserID != uid {
		return ErrCSVAccess
	}
	if row.Status != csvexport.StatusGenerationFailed && row.Status != csvexport.StatusStorageFailed {
		return ErrCSVNotReady
	}
	return s.Retry(ctx, uid, id)
}
