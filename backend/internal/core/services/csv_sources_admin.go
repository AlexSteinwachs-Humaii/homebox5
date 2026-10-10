package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/attachment"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entity"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/group"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
)

// CSVSources dispatches every catalogue entry. It is a capture authority and a
// scalar row loader, not a worker, history API or CSV encoder.
type CSVSources struct {
	db         *ent.Client
	boundaries CSVBoundaryAuthority
}

func NewCSVSources(db *ent.Client, boundaries CSVBoundaryAuthority) *CSVSources {
	return &CSVSources{db, boundaries}
}
func (s *CSVSources) AuthorizeCSVCapture(ctx context.Context, uid uuid.UUID, r CSVCaptureRequest, spec CSVSourceSpec) error {
	if spec.Source != r.Source {
		return errors.New("csv: source mismatch")
	}
	_, err := s.Load(ctx, uid, r)
	return err
}
func csvInventoryAdapter(a CSVAdapter) bool {
	return slices.Contains([]CSVAdapter{CSVEntities, CSVTags, CSVEntityTypes, CSVTemplates, CSVMaintenance}, a)
}

var errCSVAccess = errors.New("csv: inaccessible or stale source reference")

// csvSourceAccess checks owner and user restrictions independently of membership.
// Account-private/cross-collection requests never fall back to a default group.
func csvSourceAccess(ctx context.Context, repos *repo.AllRepos, uid uuid.UUID, r CSVCaptureRequest, spec CSVSourceSpec) error {
	if uid == uuid.Nil {
		return errCSVAccess
	}
	if _, err := repos.Users.GetOneID(ctx, uid); err != nil {
		return err
	}
	if spec.Scope != CSVCollection {
		return nil
	}
	member, err := repos.Groups.IsMember(ctx, *r.CollectionID, uid)
	if err != nil {
		return err
	}
	if !member {
		return errCSVAccess
	}
	if spec.Access == CSVOwner {
		owner, err := repos.Groups.IsOwnerOf(ctx, uid, *r.CollectionID)
		if err != nil {
			return err
		}
		if !owner {
			return errCSVAccess
		}
	}
	return nil
}

func (s *CSVSources) Load(ctx context.Context, uid uuid.UUID, r CSVCaptureRequest) (CSVInventoryRows, error) {
	if err := r.Validate(); err != nil {
		return CSVInventoryRows{}, err
	}
	spec, _ := CSVSourceDefinition(r.Source)
	if s.db == nil || uid == uuid.Nil {
		return CSVInventoryRows{}, errCSVAccess
	}
	if csvInventoryAdapter(spec.Adapter) {
		return NewCSVInventorySources(s.db, s.boundaries).Load(ctx, uid, r)
	}
	if r.Selector != nil || r.Recent != nil {
		if s.boundaries == nil {
			return CSVInventoryRows{}, errors.New("csv: trusted boundary authority required")
		}
		if err := s.boundaries.AuthorizeCSVBoundary(ctx, uid, r); err != nil {
			return CSVInventoryRows{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return CSVInventoryRows{}, err
	}
	defer func() { _ = tx.Rollback() }()
	db := tx.Client()
	repos := repo.New(db, nil, config.Storage{}, "", config.Thumbnail{})
	if err := csvSourceAccess(ctx, repos, uid, r, spec); err != nil {
		return CSVInventoryRows{}, err
	}
	gid := uuid.Nil
	if r.CollectionID != nil {
		gid = *r.CollectionID
	}
	records := []csvInventoryRecord{}
	add := func(id uuid.UUID, v map[CSVColumn]any) {
		v["id"] = id.String()
		records = append(records, csvInventoryRecord{id, v})
	}
	switch spec.Adapter {
	case CSVMembers:
		rows, err := repos.Users.GetUsersByGroupID(ctx, gid)
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, v := range rows {
			add(v.ID, map[CSVColumn]any{"name": v.Name, "email": v.Email})
		}
	case CSVInvitations:
		rows, err := repos.Groups.InvitationGetAll(ctx, gid)
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, v := range rows {
			add(v.ID, map[CSVColumn]any{"expiresAt": v.ExpiresAt, "uses": int64(v.Uses)})
		}
	case CSVNotifiers:
		rows, err := repos.Notifiers.GetByUser(ctx, uid, gid)
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, v := range rows {
			add(v.ID, map[CSVColumn]any{"name": v.Name, "isActive": v.IsActive, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt})
		}
	case CSVAttachments:
		exists, err := db.Entity.Query().Where(entity.ID(r.Hierarchy.RootID), entity.HasGroupWith(group.ID(gid))).Exist(ctx)
		if err != nil {
			return CSVInventoryRows{}, err
		}
		if !exists {
			return CSVInventoryRows{}, errCSVAccess
		}
		q := db.Attachment.Query().Where(attachment.HasEntityWith(entity.ID(r.Hierarchy.RootID), entity.HasGroupWith(group.ID(gid))))
		if r.Source == "entity-media" {
			q.Where(attachment.TypeEQ(attachment.TypePhoto))
		}
		// Select only metadata; do not load paths, thumbnail objects or blob URLs.
		rows, err := q.Select(attachment.FieldID, attachment.FieldTitle, attachment.FieldType, attachment.FieldMimeType, attachment.FieldPrimary).All(ctx)
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, v := range rows {
			add(v.ID, map[CSVColumn]any{"title": v.Title, "type": string(v.Type), "mimeType": v.MimeType, "primary": v.Primary})
		}
	case CSVBackups:
		rows, err := repos.Exports.ListByGroup(ctx, gid)
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, v := range rows {
			add(v.ID, map[CSVColumn]any{"kind": v.Kind, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt, "status": v.Status, "progress": int64(v.Progress), "sizeBytes": v.SizeBytes})
		}
	case CSVAPIKeys:
		rows, err := repos.APIKeys.GetByUser(ctx, uid)
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, v := range rows {
			add(v.ID, map[CSVColumn]any{"name": v.Name, "createdAt": v.CreatedAt, "expiresAt": csvNullableTime(v.ExpiresAt), "lastUsedAt": csvNullableTime(v.LastUsedAt)})
		}
	case CSVCollections:
		rows, err := repos.Groups.GetAllGroups(ctx, uid)
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, v := range rows {
			add(v.ID, map[CSVColumn]any{"name": v.Name, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt, "currency": v.Currency})
		}
	case CSVArtifacts:
		var rows []*ent.CSVExport
		if r.Scope == CSVCollection {
			rows, err = repos.CSVExports.ListByGroup(ctx, gid)
		} else {
			rows, err = repos.CSVExports.ListPrivate(ctx, uid)
		}
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, v := range rows {
			if err := s.authorizeHistoryRow(ctx, db, repos, uid, v); err != nil {
				// Permission/stale capture denies the whole export, not partial success.
				return CSVInventoryRows{}, err
			}
			add(v.ID, map[CSVColumn]any{"source": v.Source, "status": string(v.Status), "createdAt": v.CreatedAt, "completedAt": csvNullableTime(v.CompletedAt), "expiresAt": csvNullableTime(v.ExpiresAt)})
		}
	default:
		return CSVInventoryRows{}, errors.New("csv: unsupported source adapter")
	}
	if r.Selector != nil {
		universe := map[uuid.UUID]bool{}
		for _, v := range records {
			universe[v.id] = true
		}
		for _, id := range r.Selector.EligibleIDs {
			if !universe[id] {
				return CSVInventoryRows{}, errCSVAccess
			}
		}
	}
	out, err := csvProjectRecords(r, spec, records)
	if err != nil {
		return CSVInventoryRows{}, err
	}
	if err := tx.Commit(); err != nil {
		return CSVInventoryRows{}, err
	}
	return out, nil
}
func csvNullableTime(t *time.Time) any {
	if t == nil {
		return ""
	}
	return *t
}

// Recheck the artifact's source class and captured references without recursively
// exporting history. A malformed/unknown capture is never readable metadata.
func (s *CSVSources) authorizeHistoryRow(ctx context.Context, db *ent.Client, repos *repo.AllRepos, uid uuid.UUID, v *ent.CSVExport) error {
	b, err := json.Marshal(v.Capture)
	if err != nil {
		return err
	}
	r, err := DecodeCSVCapture(bytes.NewReader(b))
	if err != nil {
		return err
	}
	spec, _ := CSVSourceDefinition(r.Source)
	if string(r.Source) != v.Source || string(r.Scope) != string(v.Scope) || !sameCSVGroup(r.CollectionID, v.GroupID) {
		return errCSVAccess
	}
	if spec.Scope != CSVCollection || spec.Access == CSVUser || spec.Access == CSVMemberUser {
		if uid != v.UserID {
			return errCSVAccess
		}
	}
	if err := csvSourceAccess(ctx, repos, uid, r, spec); err != nil {
		return err
	}
	if r.Selector != nil || r.Recent != nil {
		if s.boundaries == nil {
			return errCSVAccess
		}
		if err := s.boundaries.AuthorizeCSVBoundary(ctx, uid, r); err != nil {
			return err
		}
	}

	if csvInventoryAdapter(spec.Adapter) {
		_, err := NewCSVInventorySources(db, s.boundaries).loadSnapshot(ctx, db, uid, r, spec)
		return err
	}
	if spec.Adapter == CSVCollections && r.Selector != nil {
		groups, err := repos.Groups.GetAllGroups(ctx, uid)
		if err != nil {
			return err
		}
		ids := map[uuid.UUID]bool{}
		for _, g := range groups {
			ids[g.ID] = true
		}
		for _, id := range r.Selector.EligibleIDs {
			if !ids[id] {
				return errCSVAccess
			}
		}
	}
	if spec.Adapter == CSVAttachments {
		ok, err := db.Entity.Query().Where(entity.ID(r.Hierarchy.RootID), entity.HasGroupWith(group.ID(*r.CollectionID))).Exist(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return errCSVAccess
		}
	}

	return nil
}
func sameCSVGroup(a, b *uuid.UUID) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
