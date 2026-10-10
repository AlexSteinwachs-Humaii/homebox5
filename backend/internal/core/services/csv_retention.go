package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"
)

// CSVRetentionPolicy applies to future completions only, never existing artifacts.
type CSVRetentionPolicy struct {
	Days int `json:"days" minimum:"1" maximum:"106751" default:"30" validate:"required"`
}

func (s *CSVExportService) policyAccess(ctx context.Context, uid uuid.UUID, gid *uuid.UUID) error {
	if err := s.lifecycleScope(ctx, uid, gid); err != nil {
		return err
	}
	if gid != nil {
		owner, err := s.repos.Groups.IsOwnerOf(ctx, uid, *gid)
		if err != nil {
			return err
		}
		if !owner {
			return ErrCSVAccess
		}
	}
	return nil
}

func (s *CSVExportService) Retention(ctx context.Context, uid uuid.UUID, gid *uuid.UUID) (CSVRetentionPolicy, error) {
	if err := s.policyAccess(ctx, uid, gid); err != nil {
		return CSVRetentionPolicy{}, err
	}
	var days int
	var err error
	if gid == nil {
		days, err = s.repos.CSVExports.UserRetention(ctx, uid)
	} else {
		days, err = s.repos.CSVExports.GroupRetention(ctx, *gid)
	}
	return CSVRetentionPolicy{Days: days}, err
}

func (s *CSVExportService) SetRetention(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, days int) error {
	if err := s.policyAccess(ctx, uid, gid); err != nil {
		return err
	}
	if gid == nil {
		return s.repos.CSVExports.SetUserRetention(ctx, uid, days)
	}
	return s.repos.CSVExports.SetGroupRetention(ctx, *gid, days)
}

func csvExpiry(completed time.Time, days int) (time.Time, error) {
	if err := repo.ValidateCSVRetentionDays(days); err != nil {
		return time.Time{}, err
	}
	return completed.UTC().Add(time.Duration(days) * (24 * time.Hour)), nil
}

func (s *CSVExportService) deleteBlob(ctx context.Context, key string) error {
	if s.remove != nil {
		return s.remove(ctx, key)
	}
	bucket, err := blob.OpenBucket(ctx, s.repos.Attachments.GetConnString())
	if err != nil {
		return err
	}
	defer func() { _ = bucket.Close() }()
	err = bucket.Delete(ctx, s.repos.Attachments.GetFullPath(key))
	if gcerrors.Code(err) == gcerrors.NotFound {
		return nil
	}
	return err
}

// Cleanup commits a tombstone before touching storage. The same database write
// lock used by publication fences concurrent cleanup, deletion and job retries.
// Rollback after a blob/SQL error leaves the immutable key available to retry.
func (s *CSVExportService) Cleanup(ctx context.Context) error {
	rows, err := s.repos.CSVExports.CleanupCandidates(ctx, s.now().UTC())
	if err != nil {
		return err
	}
	var errs []error
	for _, row := range rows {
		if ctx.Err() != nil {
			return errors.Join(append(errs, ctx.Err())...)
		}
		if err := s.cleanupArtifact(ctx, row.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *CSVExportService) cleanupArtifact(ctx context.Context, id uuid.UUID) error {
	now := s.now().UTC()
	_, err := s.db.CSVExport.Update().Where(csvexport.IDEQ(id), csvexport.StatusEQ(csvexport.StatusCompleted), csvexport.ExpiresAtLTE(now)).SetStatus(csvexport.StatusCleanupPending).ClearRetryAt().Save(ctx)
	if err != nil {
		return err
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	n, err := tx.CSVExport.Update().Where(csvexport.IDEQ(id), csvexport.StatusEQ(csvexport.StatusCleanupPending), csvexport.Or(csvexport.RetryAtIsNil(), csvexport.RetryAtLTE(now))).SetRetryAt(now.Add(csvRetryDelay)).Save(ctx)
	if err != nil || n == 0 {
		return err
	}
	row, err := tx.CSVExport.Get(ctx, id)
	if err == nil && row.BlobKey != fmt.Sprintf("csv/%s/%s.csv", row.UserID, row.ID) {
		err = ErrCSVAccess
	}
	if err == nil {
		err = s.deleteBlob(ctx, row.BlobKey)
	}
	if gcerrors.Code(err) == gcerrors.NotFound {
		err = nil
	}
	if err == nil {
		err = tx.CSVExport.DeleteOneID(id).Exec(ctx)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err == nil {
		return nil
	}
	_ = tx.Rollback()
	// Never regress a completed removal, even if commit acknowledgement was lost.
	_, retryErr := s.db.CSVExport.Update().Where(csvexport.IDEQ(id), csvexport.StatusEQ(csvexport.StatusCleanupPending)).SetRetryAt(now.Add(csvRetryDelay)).SetCleanupError("cleanup failed; retry scheduled").Save(ctx)
	return errors.Join(err, retryErr)
}
