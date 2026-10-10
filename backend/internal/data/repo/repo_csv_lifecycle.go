package repo

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
)

// LifecycleClient is internal plumbing for the CSV worker, never an HTTP query.
func (r *CsvExportRepository) LifecycleClient() *ent.Client { return r.db }

// AuthorizeArtifact revalidates the captured source against the current actor.
// Private scope requires ownership even for collection owners. Source references
// are checked too (e.g. scanner results from collections the user has left).
func (r *CsvExportRepository) AuthorizeArtifact(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, row *ent.CsvExport) error {
	if row.Scope == csvexport.ScopePrivate {
		if gid != nil || row.RequesterID != uid {
			return errors.New("CSV access denied")
		}
	} else if gid == nil || row.GroupID == nil || *gid != *row.GroupID {
		return errors.New("CSV access denied")
	}
	if row.AccessRequirement == "requester-member" && row.RequesterID != uid {
		return errors.New("CSV access denied")
	}
	_, err := r.ResolveSource(ctx, uid, row.GroupID, row.RequestSnapshot)
	return err
}

func (r *CsvExportRepository) IsOwner(ctx context.Context, uid, gid uuid.UUID) error {
	return (csvSourceReader{db: r.db, uid: uid}).owner(ctx, gid)
}
