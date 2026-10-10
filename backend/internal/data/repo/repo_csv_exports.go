package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
)

// MaxCSVRetentionDays is the largest whole-day count representable by time.Duration.
const MaxCSVRetentionDays = 106751
const DefaultCSVRetentionDays = 30

// CSVExportRepository stores only CSV artifacts. Access to current source data and
// membership must be checked by the service; these queries enforce history scope.
// No method accepts a caller-selected blob key. Worker state is persisted through
// Ent mutations; uploaded keys must remain recorded until cleanup succeeds.
type CSVExportRepository struct{ db *ent.Client }

// Create requires an already validated source capture (the service owns the typed
// source catalogue). The immutable specification is separate from worker state.
func (r *CSVExportRepository) Create(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, scope csvexport.Scope, source string, capture map[string]interface{}) (*ent.CSVExport, error) {
	if uid == uuid.Nil || source == "" || len(capture) == 0 {
		return nil, fmt.Errorf("CSV owner, source and capture are required")
	}
	switch scope {
	case csvexport.ScopeCollection:
		if gid == nil || *gid == uuid.Nil {
			return nil, fmt.Errorf("collection CSV requires a collection")
		}
	case csvexport.ScopeAccountPrivate, csvexport.ScopeCrossCollection:
		if gid != nil {
			return nil, fmt.Errorf("private CSV cannot have a collection")
		}
	default:
		return nil, fmt.Errorf("invalid CSV scope")
	}
	id := uuid.New()
	return r.db.CSVExport.Create().SetID(id).SetUserID(uid).SetNillableGroupID(gid).
		SetScope(scope).SetSource(source).SetCapture(capture).
		SetBlobKey(fmt.Sprintf("csv/%s/%s.csv", uid, id)).Save(ctx)
}

func (r *CSVExportRepository) ListByGroup(ctx context.Context, gid uuid.UUID) ([]*ent.CSVExport, error) {
	return r.db.CSVExport.Query().Where(csvexport.ScopeEQ(csvexport.ScopeCollection), csvexport.GroupIDEQ(gid)).Order(ent.Desc(csvexport.FieldCreatedAt), ent.Desc(csvexport.FieldID)).All(ctx)
}
func (r *CSVExportRepository) GetByGroup(ctx context.Context, gid, id uuid.UUID) (*ent.CSVExport, error) {
	return r.db.CSVExport.Query().Where(csvexport.IDEQ(id), csvexport.ScopeEQ(csvexport.ScopeCollection), csvexport.GroupIDEQ(gid)).Only(ctx)
}
func (r *CSVExportRepository) ListPrivate(ctx context.Context, uid uuid.UUID) ([]*ent.CSVExport, error) {
	return r.db.CSVExport.Query().Where(csvexport.UserIDEQ(uid), csvexport.GroupIDIsNil(), csvexport.ScopeIn(csvexport.ScopeAccountPrivate, csvexport.ScopeCrossCollection)).Order(ent.Desc(csvexport.FieldCreatedAt), ent.Desc(csvexport.FieldID)).All(ctx)
}
func (r *CSVExportRepository) GetPrivate(ctx context.Context, uid, id uuid.UUID) (*ent.CSVExport, error) {
	return r.db.CSVExport.Query().Where(csvexport.IDEQ(id), csvexport.UserIDEQ(uid), csvexport.GroupIDIsNil(), csvexport.ScopeIn(csvexport.ScopeAccountPrivate, csvexport.ScopeCrossCollection)).Only(ctx)
}

// CleanupCandidates includes expired completed artifacts and recoverable deletion
// intents. Metadata is not removed here: the worker must confirm blob deletion.
func (r *CSVExportRepository) CleanupCandidates(ctx context.Context, now time.Time) ([]*ent.CSVExport, error) {
	return r.db.CSVExport.Query().Where(csvexport.Or(
		csvexport.And(csvexport.StatusEQ(csvexport.StatusCompleted), csvexport.ExpiresAtLTE(now)),
		csvexport.And(csvexport.StatusEQ(csvexport.StatusCleanupPending), csvexport.Or(csvexport.RetryAtIsNil(), csvexport.RetryAtLTE(now))),
	)).All(ctx)
}

func ValidateCSVRetentionDays(days int) error {
	if days < 1 || days > MaxCSVRetentionDays {
		return fmt.Errorf("CSV retention must be between 1 and %d whole days", MaxCSVRetentionDays)
	}
	return nil
}
func (r *CSVExportRepository) UserRetention(ctx context.Context, uid uuid.UUID) (int, error) {
	u, err := r.db.User.Get(ctx, uid)
	if err != nil {
		return 0, err
	}
	return u.CsvRetentionDays, nil
}
func (r *CSVExportRepository) GroupRetention(ctx context.Context, gid uuid.UUID) (int, error) {
	g, err := r.db.Group.Get(ctx, gid)
	if err != nil {
		return 0, err
	}
	return g.CsvRetentionDays, nil
}

// Policy mutation is a persistence primitive. The policy service must authorize
// the current collection owner or requesting account user before calling it.
func (r *CSVExportRepository) SetUserRetention(ctx context.Context, uid uuid.UUID, days int) error {
	if err := ValidateCSVRetentionDays(days); err != nil {
		return err
	}
	return r.db.User.UpdateOneID(uid).SetCsvRetentionDays(days).Exec(ctx)
}
func (r *CSVExportRepository) SetGroupRetention(ctx context.Context, gid uuid.UUID, days int) error {
	if err := ValidateCSVRetentionDays(days); err != nil {
		return err
	}
	return r.db.Group.UpdateOneID(gid).SetCsvRetentionDays(days).Exec(ctx)
}
