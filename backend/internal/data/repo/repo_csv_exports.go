package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/group"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/user"
)

// CsvExportRepository persists contracts, not generation or storage operations.
// Callers MUST supply trusted auth identities and authorize the source first.
// Collection listing/download source revalidation belongs to the lifecycle service.
type CsvExportRepository struct{ db *ent.Client }

func (r *CsvExportRepository) Create(ctx context.Context, requester uuid.UUID, gid *uuid.UUID, req csvexports.Request) (*ent.CsvExport, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if requester == uuid.Nil || (req.Scope == "collection") != (gid != nil) || gid != nil && *gid == uuid.Nil {
		return nil, errors.New("invalid trusted scope")
	}
	source, _ := csvexports.Lookup(req.Source)
	days, err := r.Retention(ctx, requester, gid)
	if err != nil {
		return nil, err
	}
	return r.db.CsvExport.Create().SetRequesterID(requester).SetNillableGroupID(gid).
		SetScope(csvexport.Scope(req.Scope)).SetSource(req.Source).SetAccessRequirement(source.Access).
		SetRequestSnapshot(req).SetRetentionDays(days).Save(ctx)
}

func (r *CsvExportRepository) Retention(ctx context.Context, uid uuid.UUID, gid *uuid.UUID) (int, error) {
	if gid != nil {
		g, err := r.db.Group.Get(ctx, *gid)
		if err != nil {
			return 0, err
		}
		return g.CsvExportRetentionDays, nil
	}
	u, err := r.db.User.Get(ctx, uid)
	if err != nil {
		return 0, err
	}
	return u.CsvExportRetentionDays, nil
}

// Collection caller must enforce ownership; private caller must use authenticated UID.
func (r *CsvExportRepository) SetRetention(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, settings csvexports.RetentionSettings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	if gid != nil {
		return r.db.Group.UpdateOneID(*gid).SetCsvExportRetentionDays(settings.Days).Exec(ctx)
	}
	return r.db.User.UpdateOneID(uid).SetCsvExportRetentionDays(settings.Days).Exec(ctx)
}

// Scoped queries are building blocks only, not public history endpoints. They
// prevent private rows ever entering shared history. Source permission filtering
// must precede mapping metadata or serving bytes in the lifecycle service.
func (r *CsvExportRepository) CollectionQuery(gid uuid.UUID) *ent.CsvExportQuery {
	return r.db.CsvExport.Query().Where(csvexport.ScopeEQ(csvexport.ScopeCollection), csvexport.HasGroupWith(group.ID(gid)))
}
func (r *CsvExportRepository) PrivateQuery(uid uuid.UUID) *ent.CsvExportQuery {
	return r.db.CsvExport.Query().Where(csvexport.ScopeEQ(csvexport.ScopePrivate), csvexport.GroupIDIsNil(), csvexport.HasRequesterWith(user.ID(uid)))
}

// CsvExportMetadata projects a row into the public contract. In particular it
// never returns artifact_path; authorized artifact-ID download routes will be
// responsible for streaming stored bytes, not handing clients storage keys.
func CsvExportMetadata(row *ent.CsvExport) csvexports.Artifact {
	return csvexports.Artifact{
		ID: row.ID, RequesterID: row.RequesterID, GroupID: row.GroupID,
		Scope: string(row.Scope), Source: row.Source, AccessRequirement: row.AccessRequirement,
		Request: row.RequestSnapshot, RetentionDays: row.RetentionDays,
		CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, ExpiresAt: row.ExpiresAt,
		Status: string(row.Status), SizeBytes: row.SizeBytes, Error: row.Error,
	}
}
