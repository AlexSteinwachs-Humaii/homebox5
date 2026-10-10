package services

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
)

// CSVInventoryRows is a typed scalar projection, not CSV bytes. Numeric and
// boolean values remain typed for the canonical writer's formula policy.
type CSVInventoryRows struct {
	Columns []CSVColumn
	Rows    [][]any
}

// CSVBoundaryAuthority verifies trusted selector-operation eligibility and recent
// capture provenance. Raw client ID assertions alone are never sufficient. The
// API's boundary implementation belongs to the capture/lifecycle integration.
type CSVBoundaryAuthority interface {
	AuthorizeCSVBoundary(context.Context, uuid.UUID, CSVCaptureRequest) error
}

type CSVInventorySources struct {
	db         *ent.Client
	boundaries CSVBoundaryAuthority
}

func NewCSVInventorySources(db *ent.Client, boundaries CSVBoundaryAuthority) *CSVInventorySources {
	return &CSVInventorySources{db: db, boundaries: boundaries}
}

// AuthorizeCSVCapture implements the catalogue authority for inventory sources.
// Load always rechecks authorization; callers cannot use this as a durable grant.
func (s *CSVInventorySources) AuthorizeCSVCapture(ctx context.Context, uid uuid.UUID, r CSVCaptureRequest, spec CSVSourceSpec) error {
	if spec.Source != r.Source {
		return errors.New("csv: source mismatch")
	}
	_, err := s.Load(ctx, uid, r)
	return err
}

type csvInventoryRecord struct {
	id     uuid.UUID
	values map[CSVColumn]any
}

// Load materializes the complete result inside a repeatable-read transaction.
// No offset traversal, page cap, depth cap or best-effort partial success. All
// values and graph relationships come from the same snapshot. Authorization is
// current at that snapshot; download must recheck it separately.
func (s *CSVInventorySources) Load(ctx context.Context, uid uuid.UUID, r CSVCaptureRequest) (CSVInventoryRows, error) {
	fail := func() (CSVInventoryRows, error) {
		return CSVInventoryRows{}, errors.New("csv: inaccessible or stale source reference")
	}
	if err := r.Validate(); err != nil {
		return CSVInventoryRows{}, err
	}
	spec, _ := CSVSourceDefinition(r.Source)
	if !slices.Contains([]CSVAdapter{CSVEntities, CSVTags, CSVEntityTypes, CSVTemplates, CSVMaintenance}, spec.Adapter) || uid == uuid.Nil || s.db == nil {
		return fail()
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
	out, err := s.loadSnapshot(ctx, tx.Client(), uid, r, spec)
	if err != nil {
		return CSVInventoryRows{}, err
	}
	if err := tx.Commit(); err != nil {
		return CSVInventoryRows{}, err
	}
	return out, nil
}

// loadSnapshot is also used to authorize artifact metadata against its captured
// inventory source without nesting transactions or changing the read snapshot.
func (s *CSVInventorySources) loadSnapshot(ctx context.Context, db *ent.Client, uid uuid.UUID, r CSVCaptureRequest, spec CSVSourceSpec) (CSVInventoryRows, error) {
	fail := func() (CSVInventoryRows, error) { return CSVInventoryRows{}, errCSVAccess }
	repos := repo.New(db, nil, config.Storage{}, "", config.Thumbnail{})
	gid := *r.CollectionID
	member, err := repos.Groups.IsMember(ctx, gid, uid)
	if err != nil {
		return CSVInventoryRows{}, err
	}
	if !member {
		return fail()
	}
	// Full scoped graphs are also used to validate flattened relationships. Do not
	// follow repository path/tree methods with unscoped recursion or depth limits.
	all, err := repos.Entities.GetAll(ctx, gid)
	if err != nil {
		return CSVInventoryRows{}, err
	}
	tags, err := repos.Tags.GetAll(ctx, gid)
	if err != nil {
		return CSVInventoryRows{}, err
	}
	types, err := repos.EntityTypes.GetAll(ctx, gid)
	if err != nil {
		return CSVInventoryRows{}, err
	}
	ep, tp := map[uuid.UUID]uuid.UUID{}, map[uuid.UUID]uuid.UUID{}
	entities := map[uuid.UUID]repo.EntityOut{}
	typeIDs := map[uuid.UUID]bool{}
	for _, t := range types {
		typeIDs[t.ID] = true
	}
	for _, e := range all {
		entities[e.ID] = e
		ep[e.ID] = uuid.Nil
		if e.Parent != nil {
			ep[e.ID] = e.Parent.ID
		}
		if e.EntityType != nil && !typeIDs[e.EntityType.ID] {
			return fail()
		}
	}
	for _, t := range tags {
		tp[t.ID] = t.ParentID
	}
	if err := csvValidateGraph(ep); err != nil {
		return CSVInventoryRows{}, err
	}
	if err := csvValidateGraph(tp); err != nil {
		return CSVInventoryRows{}, err
	}
	for _, e := range all {
		for _, t := range e.Tags {
			if _, ok := tp[t.ID]; !ok {
				return fail()
			}
		}
	}
	check := func(ids []uuid.UUID, graph map[uuid.UUID]uuid.UUID) bool {
		for _, id := range ids {
			if _, ok := graph[id]; !ok {
				return false
			}
		}
		return true
	}
	if r.Entities != nil {
		f := r.Entities
		if !check(f.ParentIDs, ep) || !check(f.ParentItemIDs, ep) || !check(f.TagIDs, tp) {
			return fail()
		}
		for _, id := range f.ParentItemIDs {
			e := entities[id]
			if e.EntityType != nil && e.EntityType.IsLocation {
				return fail()
			}
		}
	}
	if r.Hierarchy != nil {
		graph := ep
		if r.Hierarchy.Kind == "tag" {
			graph = tp
		}
		if !check([]uuid.UUID{r.Hierarchy.RootID}, graph) {
			return fail()
		}
	}
	location := func(id uuid.UUID) string {
		for p := ep[id]; p != uuid.Nil; p = ep[p] {
			e := entities[p]
			if e.EntityType != nil && e.EntityType.IsLocation {
				return e.Name
			}
		}
		return ""
	}
	records := []csvInventoryRecord{}
	add := func(id uuid.UUID, v map[CSVColumn]any) {
		v["id"] = id.String()
		records = append(records, csvInventoryRecord{id, v})
	}
	switch spec.Adapter {
	case CSVEntities:
		f := CSVEntityFilters{}
		if r.Entities != nil {
			f = *r.Entities
		}
		q := repo.EntityQuery{Search: f.Search, ParentIDs: f.ParentIDs, ParentItemIDs: f.ParentItemIDs, TagIDs: f.TagIDs, NegateTags: f.NegateTags, OnlyWithPhoto: f.OnlyWithPhoto, OnlyWithoutPhoto: f.OnlyWithoutPhoto, IncludeArchived: f.IncludeArchived, FilterChildren: f.FilterChildren}
		if f.AssetID != "" {
			q.AssetID, _ = repo.ParseAssetID(f.AssetID)
		}
		for _, field := range f.Fields {
			q.Fields = append(q.Fields, repo.FieldQuery{Name: field.Name, Value: field.Value})
		}
		kinds := []bool{false}
		switch r.Source {
		case "locations", "location-selector":
			kinds = []bool{true}
		case "entity-hierarchy", "entity-inventory", "entity-selector":
			kinds = []bool{false, true}
		}
		if f.IsLocation != nil {
			kinds = []bool{*f.IsLocation}
		}
		for _, kind := range kinds {
			q.IsLocation = &kind
			rows, err := repos.Entities.QueryAllForCSV(ctx, gid, q)
			if err != nil {
				return CSVInventoryRows{}, err
			}
			for _, e := range rows {
				if r.Hierarchy != nil {
					h := r.Hierarchy
					if h.Kind == "entity" && !csvWithin(ep, e.ID, h.RootID, h.Descendants) {
						continue
					}
					if h.Kind == "tag" {
						matched := false
						for _, t := range e.Tags {
							if t.ID == h.RootID || h.Descendants && csvWithin(tp, t.ID, h.RootID, true) {
								matched = true
							}
						}
						if !matched {
							continue
						}
					}
				}
				names := []string{}
				for _, t := range e.Tags {
					names = append(names, t.Name)
				}
				slices.Sort(names)
				et := ""
				if e.EntityType != nil {
					et = e.EntityType.Name
				}
				add(e.ID, map[CSVColumn]any{"assetId": int64(e.AssetID), "name": e.Name, "description": e.Description, "quantity": e.Quantity, "insured": e.Insured, "archived": e.Archived, "createdAt": e.CreatedAt, "updatedAt": e.UpdatedAt, "purchasePrice": e.PurchasePrice, "parentId": csvOptionalID(ep[e.ID]), "entityType": et, "tags": strings.Join(names, "; "), "soldDate": e.SoldDate.String(), "itemCount": e.ItemCount, "location": location(e.ID)})
			}
		}
	case CSVTags:
		for _, t := range tags {
			if r.Hierarchy != nil && !csvWithin(tp, t.ID, r.Hierarchy.RootID, r.Hierarchy.Descendants) {
				continue
			}
			add(t.ID, map[CSVColumn]any{"name": t.Name, "description": t.Description, "parentId": csvOptionalID(t.ParentID), "color": t.Color, "icon": t.Icon, "createdAt": t.CreatedAt, "updatedAt": t.UpdatedAt})
		}
	case CSVEntityTypes:
		for _, t := range types {
			add(t.ID, map[CSVColumn]any{"name": t.Name, "description": t.Description, "isLocation": t.IsLocation})
		}
	case CSVTemplates:
		templates, err := repos.EntityTemplates.GetAll(ctx, gid)
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, t := range templates {
			add(t.ID, map[CSVColumn]any{"name": t.Name, "description": t.Description})
		}
	case CSVMaintenance:
		status := repo.MaintenanceFilterStatusBoth
		if r.Maintenance != nil {
			status = repo.MaintenanceFilterStatus(r.Maintenance.Status)
		}
		entries, err := repos.MaintEntry.GetAllMaintenance(ctx, gid, repo.MaintenanceFilters{Status: status})
		if err != nil {
			return CSVInventoryRows{}, err
		}
		for _, e := range entries {
			if r.Hierarchy != nil && e.ItemID != r.Hierarchy.RootID {
				continue
			}
			add(e.ID, map[CSVColumn]any{"name": e.Name, "description": e.Description, "itemId": e.ItemID.String(), "itemName": e.ItemName, "scheduledDate": e.ScheduledDate.String(), "completedDate": e.CompletedDate.String(), "cost": e.Cost})
		}
	}
	// Validate IDs against the source universe BEFORE text filtering/exclusions.
	// Entity eligible/recent references may be removed by current query predicates,
	// but deleted, foreign or wrong-kind references must fail, not disappear.
	universe := map[uuid.UUID]bool{}
	if spec.Adapter == CSVEntities {
		for id, e := range entities {
			isLocation := e.EntityType != nil && e.EntityType.IsLocation
			if (r.Source == "item-selector" || r.Source == "dashboard-recent-items") && isLocation || r.Source == "location-selector" && !isLocation {
				continue
			}
			universe[id] = true
		}
	} else {
		for _, v := range records {
			universe[v.id] = true
		}
	}
	if r.Selector != nil {
		for _, id := range r.Selector.EligibleIDs {
			if !universe[id] {
				return fail()
			}
		}
	}
	if r.Recent != nil {
		for _, id := range r.Recent.RecordIDs {
			if !universe[id] {
				return fail()
			}
		}
	}
	out, err := csvProjectRecords(r, spec, records)
	if err != nil {
		return CSVInventoryRows{}, err
	}

	return out, nil
}

// csvProjectRecords applies common validated all-results filtering, ordering and projection.
func csvProjectRecords(r CSVCaptureRequest, spec CSVSourceSpec, records []csvInventoryRecord) (CSVInventoryRows, error) {
	eligible, excluded, recent := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	if r.Selector != nil {
		for _, id := range r.Selector.EligibleIDs {
			eligible[id] = true
		}
		for _, id := range r.Selector.ExcludedIDs {
			excluded[id] = true
		}
	}
	if r.Recent != nil {
		for _, id := range r.Recent.RecordIDs {
			recent[id] = true
		}
	}
	filtered := records[:0]
	for _, v := range records {
		if r.Text != nil && r.Text.Search != "" {
			text := fmt.Sprint(v.values["name"])
			if spec.Adapter != CSVCollections {
				text += "\n" + fmt.Sprint(v.values["description"])
			}
			text = strings.ToLower(text)
			if !strings.Contains(text, strings.ToLower(r.Text.Search)) {
				continue
			}
		}
		if r.Selector != nil && (!eligible[v.id] || excluded[v.id]) {
			continue
		}
		if r.Recent != nil && !recent[v.id] {
			continue
		}
		filtered = append(filtered, v)
	}
	order := slices.Clone(r.Ordering)
	if len(order) == 0 {
		switch {
		case r.Selector != nil: // preserve captured fuzzy-ranked eligible order
		case r.Recent != nil:
			order = []CSVOrder{{"createdAt", "desc"}}
		case spec.Adapter == CSVMaintenance:
			order = []CSVOrder{{"scheduledDate", "asc"}}
		default:
			column := CSVColumn("id")
			if slices.Contains(spec.Columns, CSVColumn("name")) {
				column = "name"
			}
			order = []CSVOrder{{column, "asc"}}
		}
	}
	rank := map[uuid.UUID]int{}
	if r.Selector != nil {
		for i, id := range r.Selector.EligibleIDs {
			rank[id] = i
		}
	}
	slices.SortFunc(filtered, func(a, b csvInventoryRecord) int {
		if len(order) == 0 && r.Selector != nil {
			return cmp.Compare(rank[a.id], rank[b.id])
		}
		for _, o := range order {
			c := csvCompare(a.values[o.Column], b.values[o.Column])
			if o.Direction == "desc" {
				c = -c
			}
			if c != 0 {
				return c
			}
		}
		return strings.Compare(a.id.String(), b.id.String())
	})
	out := CSVInventoryRows{Columns: slices.Clone(r.Columns), Rows: make([][]any, 0, len(filtered))}
	for _, record := range filtered {
		row := make([]any, len(r.Columns))
		for i, c := range r.Columns {
			value, ok := record.values[c]
			if !ok {
				return CSVInventoryRows{}, fmt.Errorf("csv: missing projection %s", c)
			}
			row[i] = value
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

func csvOptionalID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

// Reject cycles, broken and cross-collection parent edges without a depth cap.
func csvValidateGraph(parents map[uuid.UUID]uuid.UUID) error {
	done := map[uuid.UUID]bool{}
	for id := range parents {
		path := map[uuid.UUID]bool{}
		for p := id; p != uuid.Nil && !done[p]; p = parents[p] {
			if _, ok := parents[p]; !ok || path[p] {
				return errors.New("csv: invalid hierarchy")
			}
			path[p] = true
		}
		for p := range path {
			done[p] = true
		}
	}
	return nil
}
func csvWithin(parents map[uuid.UUID]uuid.UUID, id, root uuid.UUID, descendants bool) bool {
	for p := parents[id]; p != uuid.Nil; p = parents[p] {
		if p == root {
			return true
		}
		if !descendants {
			break
		}
	}
	return false
}
func csvCompare(a, b any) int {
	switch v := a.(type) {
	case float64:
		return cmp.Compare(v, b.(float64))
	case int64:
		return cmp.Compare(v, b.(int64))
	case bool:
		if v == b.(bool) {
			return 0
		}
		if v {
			return 1
		}
		return -1
	case time.Time:
		return v.Compare(b.(time.Time))
	default:
		return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
	}
}
