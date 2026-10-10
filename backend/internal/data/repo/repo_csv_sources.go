package repo

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entity"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/group"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/user"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/usergroup"
)

// ResolveSource reads all matching records in one serializable transaction.
// uid/gid come from authentication, not the request. No storage or HTTP work is
// done here; lifecycle callers can serialize these rows after the read commits.
func (r *CsvExportRepository) ResolveSource(ctx context.Context, uid uuid.UUID, gid *uuid.UUID, req csvexports.Request) ([]csvexports.Row, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if uid == uuid.Nil || req.Scope == "collection" && (gid == nil || *gid == uuid.Nil) {
		return nil, errors.New("invalid trusted CSV identity")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	reader := csvSourceReader{db: tx.Client(), uid: uid, gid: gid}
	exists, err := reader.db.User.Query().Where(user.ID(uid)).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("CSV requester unavailable")
	}
	if req.Scope == "collection" {
		if err := reader.member(ctx, *gid); err != nil {
			return nil, err
		}
	}
	rows, err := reader.read(ctx, req)
	if err != nil {
		return nil, err
	}
	rows, err = csvexports.Resolve(req, rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows, nil
}

type csvSourceReader struct {
	db  *ent.Client
	uid uuid.UUID
	gid *uuid.UUID
}

func (r csvSourceReader) member(ctx context.Context, gid uuid.UUID) error {
	ok, err := r.db.Group.Query().Where(group.ID(gid), group.HasUsersWith(user.ID(r.uid))).Exist(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("CSV source access denied")
	}
	return nil
}
func (r csvSourceReader) owner(ctx context.Context, gid uuid.UUID) error {
	ok, err := r.db.UserGroup.Query().Where(usergroup.UserID(r.uid), usergroup.GroupID(gid), usergroup.RoleEQ(usergroup.RoleOwner)).Exist(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("CSV source owner access denied")
	}
	return nil
}

// Safe DTOs are projected again by Resolve. Never marshal an Ent user, API key,
// notifier, invitation or export: those models contain credentials/blob paths.
func csvRow(v any) csvexports.Row {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	row := csvexports.Row{}
	if err := dec.Decode(&row); err != nil {
		panic(err)
	}
	return row
}
func csvRows[T any](values []T) []csvexports.Row {
	rows := make([]csvexports.Row, 0, len(values))
	for _, v := range values {
		rows = append(rows, csvRow(v))
	}
	return rows
}
func truth(v *bool) bool                                { return v != nil && *v }
func (r csvSourceReader) entityRepo() *EntityRepository { return &EntityRepository{db: r.db} }
func (r csvSourceReader) entityRow(ctx context.Context, gid, id uuid.UUID) (csvexports.Row, error) {
	e, err := r.entityRepo().GetOneByGroup(ctx, gid, id)
	if err != nil {
		return nil, err
	}
	row := csvRow(e)
	row["assetId"] = int64(e.AssetID)
	row["parentId"] = nil
	if e.Parent != nil {
		row["parentId"] = e.Parent.ID.String()
	}
	row["location"] = ""
	if e.Location != nil {
		row["location"] = e.Location.Name
	}
	names := []string{}
	for _, t := range e.Tags {
		names = append(names, t.Name)
	}
	row["tags"] = strings.Join(names, ", ")
	// Build full ancestor context without relying on the UI's expansion state.
	path := []string{e.Name}
	parent := e.Parent
	seen := map[uuid.UUID]bool{id: true}
	for parent != nil {
		if seen[parent.ID] {
			return nil, errors.New("cyclic CSV hierarchy")
		}
		seen[parent.ID] = true
		path = append([]string{parent.Name}, path...)
		p, err := r.entityRepo().GetOneByGroup(ctx, gid, parent.ID)
		if err != nil {
			return nil, err
		}
		parent = p.Parent
	}
	row["path"] = strings.Join(path, " / ")
	children, err := r.entityRepo().QueryByGroup(ctx, gid, EntityQuery{Page: -1, PageSize: -1, ParentIDs: []uuid.UUID{id}})
	if err != nil {
		return nil, err
	}
	row["childItemCount"] = children.Total
	var quantity float64
	for _, child := range children.Items {
		quantity += child.Quantity
	}
	row["itemCount"] = quantity
	row["locationChildCount"] = len(e.Children)
	var total float64
	for _, child := range children.Items {
		if !child.Archived {
			total += child.PurchasePrice
		}
	}
	row["totalPrice"] = total

	return row, nil
}
func (r csvSourceReader) read(ctx context.Context, req csvexports.Request) ([]csvexports.Row, error) {
	var gid uuid.UUID
	if r.gid != nil {
		gid = *r.gid
	}
	f := req.Filters
	switch req.Source {
	case "asset-results":
		if f.AssetID == nil {
			return nil, errors.New("asset ID required")
		}
		values, err := r.entityRepo().QueryByAssetID(ctx, gid, AssetID(*f.AssetID), -1, -1)
		if err != nil {
			return nil, err
		}
		rows := []csvexports.Row{}
		for _, e := range values.Items {
			row, err := r.entityRow(ctx, gid, e.ID)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
		return rows, nil
	case "entities", "locations":
		if req.Source == "locations" && req.Boundary.Kind == "hierarchy" {
			return r.locationHierarchy(ctx, gid, req)
		}
		q := EntityQuery{Page: -1, PageSize: -1, Search: f.Search, IsLocation: f.IsLocation, ParentIDs: f.ParentIDs, ParentItemIDs: f.ParentItemIDs, TagIDs: f.TagIDs, NegateTags: truth(f.NegateTags), OnlyWithoutPhoto: truth(f.OnlyWithoutPhoto), OnlyWithPhoto: truth(f.OnlyWithPhoto), IncludeArchived: truth(f.IncludeArchived), FilterChildren: truth(f.FilterChildren)}
		if f.AssetID != nil {
			q.AssetID = AssetID(*f.AssetID)
		}
		for _, field := range f.Fields {
			q.Fields = append(q.Fields, FieldQuery{Name: field.Name, Value: field.Value})
		}
		if req.Source == "locations" {
			yes := true
			q.IsLocation = &yes
			q.IncludeArchived = true
		}
		if f.EntityID != nil {
			if _, err := r.entityRepo().GetOneByGroup(ctx, gid, *f.EntityID); err != nil {
				return nil, err
			}
			if len(q.ParentIDs) > 0 {
				found := false
				for _, id := range q.ParentIDs {
					if id == *f.EntityID {
						found = true
					}
				}
				if !found {
					return []csvexports.Row{}, nil
				}
			}
			q.ParentIDs = []uuid.UUID{*f.EntityID}
		}
		if req.Boundary.Kind == "children" && f.EntityID == nil {
			return nil, errors.New("entity context required")
		}
		if req.Boundary.Kind == "recent" {
			q.OrderBy = "createdAt"
		}
		result, err := r.entityRepo().QueryByGroup(ctx, gid, q)
		if err != nil {
			return nil, err
		}
		if req.Boundary.Kind == "recent" && len(result.Items) > req.Boundary.Limit {
			result.Items = result.Items[:req.Boundary.Limit]
		}
		rows := []csvexports.Row{}
		for _, e := range result.Items {
			row, err := r.entityRow(ctx, gid, e.ID)
			if err != nil {
				return nil, err
			}
			row["itemCount"] = e.ItemCount
			rows = append(rows, row)
		}
		if req.Source == "locations" && truth(f.ShowItems) {
			// Include each location's direct displayed items, independent of collapse.
			locations := append([]csvexports.Row(nil), rows...)
			for _, loc := range locations {
				id, _ := uuid.Parse(fmt.Sprint(loc["id"]))
				children, err := r.entityRepo().QueryByGroup(ctx, gid, EntityQuery{Page: -1, PageSize: -1, ParentIDs: []uuid.UUID{id}})
				if err != nil {
					return nil, err
				}
				for _, e := range children.Items {
					row, err := r.entityRow(ctx, gid, e.ID)
					if err != nil {
						return nil, err
					}
					rows = append(rows, row)
				}
			}
		}
		return rows, nil
	case "scanner-results", "label-records":
		rows := []csvexports.Row{}
		for _, id := range req.Boundary.RecordIDs {
			e, err := r.db.Entity.Query().Where(entity.ID(id)).WithGroup().Only(ctx)
			if err != nil {
				return nil, err
			}
			if err := r.member(ctx, e.Edges.Group.ID); err != nil {
				return nil, err
			}
			row, err := r.entityRow(ctx, e.Edges.Group.ID, id)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
		return rows, nil
	case "templates":
		values, err := (&EntityTemplatesRepository{db: r.db}).GetAll(ctx, gid)
		return searchRows(csvRows(values), f.Search), err
	case "template-fields":
		if f.TemplateID == nil {
			return nil, errors.New("template context required")
		}
		value, err := (&EntityTemplatesRepository{db: r.db}).GetOne(ctx, gid, *f.TemplateID)
		if err != nil {
			return nil, err
		}
		rows := csvRows(value.Fields)
		for _, row := range rows {
			fieldValue(row)
		}
		return rows, nil
	case "entity-types":
		values, err := (&EntityTypeRepository{db: r.db}).GetAll(ctx, gid)
		rows := searchRows(csvRows(values), f.Search)
		out := []csvexports.Row{}
		for _, row := range rows {
			if f.IsLocation != nil && row["isLocation"] != *f.IsLocation {
				continue
			}
			if v, ok := row["defaultTemplate"].(map[string]any); ok {
				row["defaultTemplate"] = v["name"]
			} else {
				row["defaultTemplate"] = nil
			}
			out = append(out, row)
		}
		return out, err
	case "entity-fields", "attachments", "entity-tags", "entity-path":
		if req.Source == "entity-fields" && req.Boundary.Kind == "eligible" && f.EntityID == nil {
			entities, err := r.entityRepo().GetAll(ctx, gid)
			if err != nil {
				return nil, err
			}
			rows := []csvexports.Row{}
			seen := map[string]bool{}
			for _, e := range entities {
				for _, field := range e.Fields {
					if f.FieldName != "" && field.Name != f.FieldName {
						continue
					}
					row := csvRow(field)
					if field.Type == "time" {
						stored, err := r.db.Entity.Query().Where(entity.ID(e.ID)).QueryFields().All(ctx)
						if err != nil {
							return nil, err
						}
						for _, v := range stored {
							if v.ID == field.ID {
								row["timeValue"] = v.TimeValue
							}
						}
					}
					fieldValue(row)
					match := field.Name
					if f.FieldName != "" {
						match = fmt.Sprint(row["value"])
					}
					if !strings.Contains(strings.ToLower(match), strings.ToLower(f.Search)) {
						continue
					}
					key := field.Name
					if f.FieldName != "" {
						key += fmt.Sprint(row["value"])
					}
					if !seen[key] {
						seen[key] = true
						rows = append(rows, row)
					}
				}
			}
			return rows, nil
		}
		if f.EntityID == nil {
			return nil, errors.New("entity context required")
		}
		e, err := r.entityRepo().GetOneByGroup(ctx, gid, *f.EntityID)
		if err != nil {
			return nil, err
		}
		switch req.Source {
		case "entity-fields":
			rows := []csvexports.Row{}
			stored, err := r.db.Entity.Query().Where(entity.ID(e.ID)).QueryFields().All(ctx)
			if err != nil {
				return nil, err
			}
			for _, field := range e.Fields {
				row := csvRow(field)
				for _, v := range stored {
					if v.ID == field.ID {
						row["timeValue"] = v.TimeValue
					}
				}
				fieldValue(row)
				if f.FieldName != "" && field.Name != f.FieldName {
					continue
				}
				rows = append(rows, row)
			}
			return searchRows(rows, f.Search), nil
		case "attachments":
			rows := []csvexports.Row{}
			for _, a := range e.Attachments {
				row := csvRow(a)
				if f.AttachmentType != "" && fmt.Sprint(row["type"]) != f.AttachmentType {
					continue
				}
				rows = append(rows, row)
			}
			return rows, nil
		case "entity-tags":
			return r.tagRows(ctx, gid, e.Tags, false)
		default:
			rows := []csvexports.Row{}
			id := e.ID
			seen := map[uuid.UUID]bool{}
			for id != uuid.Nil {
				if seen[id] {
					return nil, errors.New("cyclic CSV path")
				}
				seen[id] = true
				row, err := r.entityRow(ctx, gid, id)
				if err != nil {
					return nil, err
				}
				rows = append([]csvexports.Row{row}, rows...)
				id = uuid.Nil
				if row["parentId"] != nil {
					id, _ = uuid.Parse(fmt.Sprint(row["parentId"]))
				}
			}
			return rows, nil
		}
	case "tags":
		if f.EntityID != nil {
			if _, err := (&TagRepository{db: r.db}).GetOneByGroup(ctx, gid, *f.EntityID); err != nil {
				return nil, err
			}
		}
		values, err := (&TagRepository{db: r.db}).GetAll(ctx, gid)
		if err != nil {
			return nil, err
		}
		if f.EntityID != nil {
			filtered := []TagSummary{}
			byID := map[uuid.UUID]TagSummary{}
			for _, v := range values {
				byID[v.ID] = v
			}
			for _, v := range values {
				p := v.ID
				seen := map[uuid.UUID]bool{}
				for p != uuid.Nil && !seen[p] {
					if p == *f.EntityID {
						filtered = append(filtered, v)
						break
					}
					seen[p] = true
					p = byID[p].ParentID
				}
			}
			values = filtered
		}
		rows, err := r.tagRows(ctx, gid, values, truth(f.ShowItems))
		if req.Boundary.Kind == "hierarchy" {
			sort.SliceStable(rows, func(i, j int) bool {
				return strings.ToLower(fmt.Sprint(rows[i]["path"])) < strings.ToLower(fmt.Sprint(rows[j]["path"]))
			})
		}
		return searchRows(rows, f.Search), err
	case "maintenance":
		if f.Status != "" && f.Status != "scheduled" && f.Status != "completed" && f.Status != "both" {
			return nil, errors.New("unsupported maintenance status")
		}
		mr := &MaintenanceEntryRepository{db: r.db}
		filters := MaintenanceFilters{Status: MaintenanceFilterStatus(f.Status)}
		if filters.Status == "" {
			filters.Status = MaintenanceFilterStatusBoth
		}
		var values []MaintenanceEntryWithDetails
		var err error
		if f.EntityID != nil {
			if _, err := r.entityRepo().GetOneByGroup(ctx, gid, *f.EntityID); err != nil {
				return nil, err
			}
			values, err = mr.GetMaintenanceByItemID(ctx, gid, *f.EntityID, filters)
		} else {
			values, err = mr.GetAllMaintenance(ctx, gid, filters)
		}
		rows := csvRows(values)
		for i, row := range rows {
			row["cost"] = values[i].Cost
			row["entityName"] = values[i].ItemName
			row["status"] = "scheduled"
			if !values[i].CompletedDate.Time().IsZero() && (f.EntityID == nil || !values[i].CompletedDate.Time().After(time.Now())) {
				row["status"] = "completed"
			}
		}
		return searchRows(rows, f.Search), err
	case "members":
		values, err := r.db.Group.Query().Where(group.ID(gid)).QueryUsers().All(ctx)
		rows := []csvexports.Row{}
		for _, v := range values {
			rows = append(rows, csvexports.Row{"id": v.ID.String(), "name": v.Name, "email": v.Email})
		}
		return rows, err
	case "invitations":
		values, err := NewGroupRepository(r.db, nil).InvitationGetAll(ctx, gid)
		rows := []csvexports.Row{}
		for _, v := range values {
			rows = append(rows, csvexports.Row{"id": v.ID.String(), "expiresAt": v.ExpiresAt, "remainingUses": v.Uses})
		}
		return rows, err
	case "notifiers":
		values, err := NewNotifierRepository(r.db).GetByUser(ctx, r.uid, gid)
		return csvRows(values), err
	case "api-keys":
		values, err := NewAPIKeyRepository(r.db).GetByUser(ctx, r.uid)
		return csvRows(values), err
	case "collections":
		values, err := r.db.UserGroup.Query().Where(usergroup.UserID(r.uid)).WithGroup().All(ctx)
		rows := []csvexports.Row{}
		for _, v := range values {
			rows = append(rows, csvexports.Row{"id": v.GroupID.String(), "name": v.Edges.Group.Name, "role": string(v.Role)})
		}
		return searchRows(rows, f.Search), err
	case "zip-history":
		values, err := (&ExportRepository{db: r.db}).ListByGroup(ctx, gid)
		rows := csvRows(values)
		for _, row := range rows {
			if row["error"] != "" {
				row["error"] = "Backup operation failed"
			}
		}
		return rows, err
	case "collection-csv-history", "private-csv-history":
		cr := &CsvExportRepository{db: r.db}
		q := cr.PrivateQuery(r.uid)
		if req.Source == "collection-csv-history" {
			q = cr.CollectionQuery(gid)
		}
		values, err := q.All(ctx)
		if err != nil {
			return nil, err
		}
		rows := []csvexports.Row{}
		for _, v := range values {
			// Do not disclose privileged source metadata through member history.
			if v.Scope == "collection" {
				if v.AccessRequirement == "collection-owner" {
					if err := r.owner(ctx, gid); err != nil {
						continue
					}
				}
				if v.AccessRequirement == "requester-member" && v.RequesterID != r.uid {
					continue
				}
			}
			rows = append(rows, csvRow(CsvExportMetadata(v)))
		}
		return rows, nil
	case "draft-fields", "draft-photos", "product-results":
		rows := csvRows(req.Snapshot)
		if req.Source == "draft-fields" {
			for i, snap := range req.Snapshot {
				switch snap.Type {
				case "number":
					if _, err := strconv.ParseFloat(snap.Value, 64); err != nil {
						return nil, errors.New("invalid draft number")
					}
					rows[i]["value"] = json.Number(snap.Value)
				case "boolean":
					b, err := strconv.ParseBool(snap.Value)
					if err != nil {
						return nil, errors.New("invalid draft boolean")
					}
					rows[i]["value"] = b
				case "time":
					if snap.Value != "" {
						dt, err := time.Parse(time.RFC3339, snap.Value)
						if err != nil {
							return nil, errors.New("invalid draft date")
						}
						rows[i]["value"] = dt
					}
				case "text", "":
				default:
					return nil, errors.New("invalid draft field type")
				}
			}
		}
		return rows, nil
	case "invitation-visible":
		// Private rows have no retained tenant. Resolve each origin through its
		// stored reference, never a client-supplied collection or token.
		rows := []csvexports.Row{}
		for _, snap := range req.Snapshot {
			if snap.ID == nil {
				return nil, errors.New("invitation reference required")
			}
			v, err := r.db.GroupInvitationToken.Get(ctx, *snap.ID)
			if err != nil {
				return nil, err
			}
			origin, err := v.QueryGroup().Only(ctx)
			if err != nil {
				return nil, err
			}
			if err := r.owner(ctx, origin.ID); err != nil {
				return nil, err
			}
			rows = append(rows, csvexports.Row{"id": v.ID.String(), "expiresAt": v.ExpiresAt, "remainingUses": v.Uses})
		}
		return rows, nil
	}
	return nil, errors.New("CSV adapter missing")
}
func fieldValue(row csvexports.Row) {
	switch row["type"] {
	case "number":
		row["value"] = row["numberValue"]
	case "boolean":
		row["value"] = row["booleanValue"]
	case "time":
		row["value"] = row["timeValue"]
	default:
		row["value"] = row["textValue"]
	}
}
func searchRows(rows []csvexports.Row, search string) []csvexports.Row {
	if search == "" {
		return rows
	}
	out := []csvexports.Row{}
	search = strings.ToLower(search)
	for _, row := range rows {
		if strings.Contains(strings.ToLower(fmt.Sprint(row["name"])), search) {
			out = append(out, row)
		}
	}
	return out
}
func (r csvSourceReader) tagRows(ctx context.Context, gid uuid.UUID, values []TagSummary, items bool) ([]csvexports.Row, error) {
	all, err := (&TagRepository{db: r.db}).GetAll(ctx, gid)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]TagSummary{}
	for _, t := range all {
		byID[t.ID] = t
	}
	rows := []csvexports.Row{}
	for _, t := range values {
		row := csvRow(t)
		path := []string{t.Name}
		p := t.ParentID
		seen := map[uuid.UUID]bool{t.ID: true}
		for p != uuid.Nil {
			if seen[p] {
				return nil, errors.New("cyclic CSV tag hierarchy")
			}
			seen[p] = true
			parent, ok := byID[p]
			if !ok {
				return nil, errors.New("CSV tag ancestor unavailable")
			}
			path = append([]string{parent.Name}, path...)
			p = parent.ParentID
		}
		row["path"] = strings.Join(path, " / ")
		result, err := r.entityRepo().QueryByGroup(ctx, gid, EntityQuery{Page: -1, PageSize: -1, TagIDs: []uuid.UUID{t.ID}})
		if err != nil {
			return nil, err
		}
		row["itemCount"] = len(result.Items)
		rows = append(rows, row)
		if items {
			for _, e := range result.Items {
				v, err := r.entityRow(ctx, gid, e.ID)
				if err != nil {
					return nil, err
				}
				v["parentId"] = t.ID.String()
				v["path"] = fmt.Sprint(row["path"]) + " / " + e.Name
				rows = append(rows, v)
			}
		}
	}
	return rows, nil
}

// locationHierarchy mirrors the tree's reachable roots/location children and
// recursively includes item children only when shown. Expansion is irrelevant.
func (r csvSourceReader) locationHierarchy(ctx context.Context, gid uuid.UUID, req csvexports.Request) ([]csvexports.Row, error) {
	all, err := r.entityRepo().GetAll(ctx, gid)
	if err != nil {
		return nil, err
	}
	byParent := map[uuid.UUID][]EntityOut{}
	for _, e := range all {
		parent := uuid.Nil
		if e.Parent != nil {
			parent = e.Parent.ID
		}
		byParent[parent] = append(byParent[parent], e)
	}
	for p := range byParent {
		sort.SliceStable(byParent[p], func(i, j int) bool {
			a, b := byParent[p][i], byParent[p][j]
			al := a.EntityType != nil && a.EntityType.IsLocation
			bl := b.EntityType != nil && b.EntityType.IsLocation
			if al != bl {
				return al
			}
			if strings.ToLower(a.Name) == strings.ToLower(b.Name) {
				return a.ID.String() < b.ID.String()
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		})
	}
	rows := []csvexports.Row{}
	seen := map[uuid.UUID]bool{}
	var visit func(uuid.UUID, bool) error
	visit = func(parent uuid.UUID, underItem bool) error {
		for _, e := range byParent[parent] {
			location := e.EntityType != nil && e.EntityType.IsLocation
			if parent == uuid.Nil && !location || underItem && location || !truth(req.Filters.ShowItems) && !location {
				continue
			}
			if seen[e.ID] {
				return errors.New("cyclic CSV hierarchy")
			}
			seen[e.ID] = true
			row, err := r.entityRow(ctx, gid, e.ID)
			if err != nil {
				return err
			}
			if strings.Contains(strings.ToLower(e.Name), strings.ToLower(req.Filters.Search)) {
				rows = append(rows, row)
			}
			if err := visit(e.ID, !location); err != nil {
				return err
			}
		}
		return nil
	}
	parent := uuid.Nil
	if req.Filters.EntityID != nil {
		parent = *req.Filters.EntityID
		if _, err := r.entityRepo().GetOneByGroup(ctx, gid, parent); err != nil {
			return nil, err
		}
	}
	if err := visit(parent, false); err != nil {
		return nil, err
	}
	return rows, nil
}
