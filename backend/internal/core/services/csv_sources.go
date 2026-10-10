package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
)

// CSVSource identifies a list surface, not a table, SQL query or blob path.
type CSVSource string
type CSVAdapter string
type CSVScope string
type CSVAccess string
type CSVColumn string

const (
	CSVCollection      CSVScope   = "collection"
	CSVAccountPrivate  CSVScope   = "account-private"
	CSVCrossCollection CSVScope   = "cross-collection"
	CSVMember          CSVAccess  = "collection-member"
	CSVOwner           CSVAccess  = "collection-owner"
	CSVUser            CSVAccess  = "requesting-user"
	CSVMemberUser      CSVAccess  = "collection-member-requesting-user"
	CSVEntities        CSVAdapter = "entities"
	CSVTags            CSVAdapter = "tags"
	CSVEntityTypes     CSVAdapter = "entity-types"
	CSVTemplates       CSVAdapter = "templates"
	CSVMaintenance     CSVAdapter = "maintenance"
	CSVMembers         CSVAdapter = "members"
	CSVInvitations     CSVAdapter = "invitations"
	CSVNotifiers       CSVAdapter = "notifiers-metadata"
	CSVAttachments     CSVAdapter = "attachments-metadata"
	CSVBackups         CSVAdapter = "backup-history"
	CSVArtifacts       CSVAdapter = "csv-history"
	CSVAPIKeys         CSVAdapter = "api-key-metadata"
	CSVCollections     CSVAdapter = "user-collections"
)

// CSVSourceSpec is the server coverage matrix. Adapter is a typed dispatch
// contract; executable row adapters are implemented by subsequent stories.
// Columns are an explicit projection, never reflection over an Ent object/DTO.
type CSVSourceSpec struct {
	Source            CSVSource   `json:"source"`
	Adapter           CSVAdapter  `json:"adapter"`
	Scope             CSVScope    `json:"scope"`
	Access            CSVAccess   `json:"access"`
	Columns           []CSVColumn `json:"columns"`
	OrderColumns      []CSVColumn `json:"orderColumns"`
	EntityFilter      bool        `json:"entityFilter"`
	MaintenanceFilter bool        `json:"maintenanceFilter"`
	TextFilter        bool        `json:"textFilter"`
	ContextKind       string      `json:"contextKind,omitempty"`
	SelectorKind      string      `json:"selectorKind,omitempty"`
	Recent            bool        `json:"recent"`
}

func csvSpecs() []CSVSourceSpec {
	entityColumns := []CSVColumn{"id", "assetId", "name", "description", "quantity", "insured", "archived", "createdAt", "updatedAt", "purchasePrice", "parentId", "entityType", "tags", "soldDate", "itemCount", "location"}
	entityOrder := []CSVColumn{"id", "name", "createdAt", "updatedAt", "assetId", "quantity", "insured", "purchasePrice", "location", "archived"}
	spec := func(source CSVSource, adapter CSVAdapter, columns []CSVColumn) CSVSourceSpec {
		order := []CSVColumn{"id"}
		if slices.Contains(columns, CSVColumn("name")) {
			order = append(order, "name")
		}
		return CSVSourceSpec{Source: source, Adapter: adapter, Scope: CSVCollection, Access: CSVMember, Columns: columns, OrderColumns: order}
	}
	var out []CSVSourceSpec
	for _, source := range []CSVSource{"items", "locations", "dashboard-recent-items", "entity-inventory", "tag-inventory", "entity-hierarchy", "item-selector", "location-selector", "entity-selector"} {
		s := spec(source, CSVEntities, entityColumns)
		s.EntityFilter = true
		s.OrderColumns = entityOrder
		switch source {
		case "dashboard-recent-items":
			s.Recent = true
		case "entity-inventory", "entity-hierarchy":
			s.ContextKind = "entity"
		case "tag-inventory":
			s.ContextKind = "tag"
		case "item-selector":
			s.SelectorKind = "item"
		case "location-selector":
			s.SelectorKind = "location"
		case "entity-selector":
			s.SelectorKind = "entity"
		}
		out = append(out, s)
	}
	for _, source := range []CSVSource{"tags", "tag-hierarchy", "tag-selector"} {
		s := spec(source, CSVTags, []CSVColumn{"id", "name", "description", "parentId", "color", "icon", "createdAt", "updatedAt"})
		s.OrderColumns = []CSVColumn{"id", "name"}
		s.TextFilter = true
		if source == "tag-hierarchy" {
			s.ContextKind = "tag"
		}
		if source == "tag-selector" {
			s.SelectorKind = "tag"
		}
		out = append(out, s)
	}
	for _, entry := range []struct {
		source   CSVSource
		adapter  CSVAdapter
		columns  []CSVColumn
		selector string
	}{
		{"entity-types", CSVEntityTypes, []CSVColumn{"id", "name", "description", "isLocation"}, ""},
		{"entity-type-selector", CSVEntityTypes, []CSVColumn{"id", "name", "description", "isLocation"}, "entity-type"},
		{"templates", CSVTemplates, []CSVColumn{"id", "name", "description"}, ""},
		{"template-selector", CSVTemplates, []CSVColumn{"id", "name", "description"}, "template"},
		{"maintenance", CSVMaintenance, []CSVColumn{"id", "name", "description", "itemId", "itemName", "scheduledDate", "completedDate", "cost"}, ""},
		{"entity-maintenance", CSVMaintenance, []CSVColumn{"id", "name", "description", "itemId", "itemName", "scheduledDate", "completedDate", "cost"}, ""},
		{"collection-members", CSVMembers, []CSVColumn{"id", "name", "email"}, ""},
		{"collection-invitations", CSVInvitations, []CSVColumn{"id", "expiresAt", "uses"}, ""},
		{"collection-notifiers", CSVNotifiers, []CSVColumn{"id", "name", "isActive", "createdAt", "updatedAt"}, ""},
		{"entity-attachments", CSVAttachments, []CSVColumn{"id", "title", "type", "mimeType", "primary"}, ""},
		{"entity-media", CSVAttachments, []CSVColumn{"id", "title", "type", "mimeType", "primary"}, ""},
		{"backup-history", CSVBackups, []CSVColumn{"id", "kind", "createdAt", "updatedAt", "status", "progress", "sizeBytes"}, ""},
		{"collection-csv-history", CSVArtifacts, []CSVColumn{"id", "source", "status", "createdAt", "completedAt", "expiresAt"}, ""},
		{"private-csv-history", CSVArtifacts, []CSVColumn{"id", "source", "status", "createdAt", "completedAt", "expiresAt"}, ""},
		{"account-api-keys", CSVAPIKeys, []CSVColumn{"id", "name", "createdAt", "expiresAt", "lastUsedAt"}, ""},
		{"account-collections", CSVCollections, []CSVColumn{"id", "name", "createdAt", "updatedAt", "currency"}, ""},
		{"collection-selector", CSVCollections, []CSVColumn{"id", "name", "createdAt", "updatedAt", "currency"}, "collection"},
	} {
		s := spec(entry.source, entry.adapter, entry.columns)
		s.SelectorKind = entry.selector
		s.TextFilter = entry.adapter == CSVEntityTypes || entry.adapter == CSVTemplates || entry.adapter == CSVCollections
		switch entry.adapter {
		case CSVMaintenance:
			s.MaintenanceFilter = true
			if entry.source == "entity-maintenance" {
				s.ContextKind = "entity"
			}
		case CSVNotifiers:
			s.Access = CSVMemberUser
		case CSVAttachments:
			s.ContextKind = "entity"
		case CSVAPIKeys:
			s.Scope = CSVAccountPrivate
			s.Access = CSVUser
		case CSVCollections:
			s.Scope = CSVCrossCollection
			s.Access = CSVUser
		}
		if entry.source == "private-csv-history" {
			s.Scope = CSVAccountPrivate
			s.Access = CSVUser
		}
		out = append(out, s)
	}
	return out
}

// CSVSourceCatalogue returns a fresh copy so consumers cannot alter policy.
func CSVSourceCatalogue() []CSVSourceSpec { return csvSpecs() }
func CSVSourceDefinition(source CSVSource) (CSVSourceSpec, bool) {
	for _, s := range csvSpecs() {
		if s.Source == source {
			return s, true
		}
	}
	return CSVSourceSpec{}, false
}

// CSVEntityFilters mirrors the data predicates of repo.EntityQuery, deliberately
// omitting Page/PageSize and sorting. nil IsLocation does NOT mean all entities:
// QueryByGroup defaults to items. The entities adapter must use explicit surface
// semantics for combined hierarchy/entity-selector traversal.
type CSVEntityFilters struct {
	Search           string           `json:"search,omitempty"`
	IsLocation       *bool            `json:"isLocation,omitempty"`
	ParentIDs        []uuid.UUID      `json:"parentIds,omitempty"`
	ParentItemIDs    []uuid.UUID      `json:"parentItemIds,omitempty"`
	TagIDs           []uuid.UUID      `json:"tagIds,omitempty"`
	Fields           []CSVFieldFilter `json:"fields,omitempty"`
	AssetID          string           `json:"assetId,omitempty"`
	NegateTags       bool             `json:"negateTags,omitempty"`
	OnlyWithoutPhoto bool             `json:"onlyWithoutPhoto,omitempty"`
	OnlyWithPhoto    bool             `json:"onlyWithPhoto,omitempty"`
	IncludeArchived  bool             `json:"includeArchived,omitempty"`
	FilterChildren   bool             `json:"filterChildren,omitempty"`
}
type CSVFieldFilter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type CSVTextFilters struct {
	Search string `json:"search"`
}

type CSVMaintenanceFilters struct {
	Status string `json:"status"`
}
type CSVOrder struct {
	Column    CSVColumn `json:"column"`
	Direction string    `json:"direction"`
}
type CSVHierarchyContext struct {
	Kind        string    `json:"kind"`
	RootID      uuid.UUID `json:"rootId"`
	Descendants bool      `json:"descendants"`
}

// Eligibility is a server-recognized selector policy, not a client predicate.
// Adapters must verify the policy/context and exclusions against the originating
// selector operation. An empty eligible set must never mean unrestricted.
type CSVSelectorCapture struct {
	Kind        string      `json:"kind"`
	Eligibility string      `json:"eligibility"`
	EligibleIDs []uuid.UUID `json:"eligibleIds"`
	ExcludedIDs []uuid.UUID `json:"excludedIds"`
}

// A recent boundary freezes membership, not values: workers read current values
// for these IDs only, recheck access, and fail on stale references. Empty IDs
// denotes an empty recent set. Neither page size nor generation time recomputes it.
type CSVRecentBoundary struct {
	CapturedAt time.Time   `json:"capturedAt"`
	RecordIDs  []uuid.UUID `json:"recordIds"`
}
type CSVCaptureRequest struct {
	Source       CSVSource              `json:"source"`
	Scope        CSVScope               `json:"scope"`
	CollectionID *uuid.UUID             `json:"collectionId,omitempty"`
	Columns      []CSVColumn            `json:"columns"`
	Ordering     []CSVOrder             `json:"ordering"`
	Entities     *CSVEntityFilters      `json:"entities,omitempty"`
	Text         *CSVTextFilters        `json:"text,omitempty"`
	Maintenance  *CSVMaintenanceFilters `json:"maintenance,omitempty"`
	Hierarchy    *CSVHierarchyContext   `json:"hierarchy,omitempty"`
	Selector     *CSVSelectorCapture    `json:"selector,omitempty"`
	Recent       *CSVRecentBoundary     `json:"recent,omitempty"`
}

// CSVCaptureAuthority is mandatory before accepting a capture. Implementations
// must resolve authenticated user identity, current source-specific access,
// membership/owner restrictions, EVERY referenced ID in its collection/user
// scope, selector operation eligibility/exclusions and server-verified recent
// membership/time. Never trust a selected/default collection, submitted role,
// eligible IDs or timestamps. Workers/download services must recheck authority.
// Missing authority is an explicit prerequisite, never permissive fallback.
type CSVCaptureAuthority interface {
	AuthorizeCSVCapture(context.Context, uuid.UUID, CSVCaptureRequest, CSVSourceSpec) error
}

func ValidateCSVCapture(ctx context.Context, userID uuid.UUID, r CSVCaptureRequest, authority CSVCaptureAuthority) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if userID == uuid.Nil || authority == nil {
		return errors.New("csv: authenticated capture authority required")
	}
	s, _ := CSVSourceDefinition(r.Source)
	return authority.AuthorizeCSVCapture(ctx, userID, r, s)
}

// DecodeCSVCapture rejects unknown keys (including pagination, tenant/role or
// blob selectors), duplicate keys, null documents and trailing JSON. API routes
// must bound the body size and then call ValidateCSVCapture, not just Validate.
func DecodeCSVCapture(reader io.Reader) (CSVCaptureRequest, error) {
	var r CSVCaptureRequest
	data, err := io.ReadAll(io.LimitReader(reader, 1<<20+1))
	if err != nil {
		return r, err
	}
	if len(data) > 1<<20 {
		return r, errors.New("csv: capture exceeds 1 MiB")
	}
	if err := csvUniqueJSON(data); err != nil {
		return r, err
	}
	if err := csvExactJSONFields(data, reflect.TypeFor[CSVCaptureRequest]()); err != nil {
		return r, err
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return r, errors.New("csv: request must be an object")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return r, err
	}
	if err := r.Validate(); err != nil {
		return CSVCaptureRequest{}, err
	}
	return r, nil
}

// encoding/json accepts case-insensitive field aliases; capture contracts do not.
func csvExactJSONFields(data []byte, typ reflect.Type) error {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	switch typ.Kind() {
	case reflect.Struct:
		if typ == reflect.TypeFor[time.Time]() {
			return nil
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil {
			return err
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			fields[strings.Split(f.Tag.Get("json"), ",")[0]] = f.Type
		}
		for k, v := range object {
			ft, ok := fields[k]
			if !ok {
				return fmt.Errorf("csv: unsupported field %s", k)
			}
			if err := csvExactJSONFields(v, ft); err != nil {
				return err
			}
		}
	case reflect.Slice:
		var values []json.RawMessage
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
		for _, v := range values {
			if err := csvExactJSONFields(v, typ.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

func csvUniqueJSON(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				k, ok := key.(string)
				if !ok || seen[k] {
					return errors.New("csv: duplicate JSON key")
				}
				seen[k] = true
				if err := walk(); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := walk(); err != nil {
					return err
				}
			}
		default:
			return errors.New("csv: invalid JSON")
		}
		_, err = d.Token()
		return err
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("csv: trailing JSON")
	}
	return nil
}

// Validate checks syntax/capabilities only, NOT authorization. Do not enqueue
// requests based on this method alone. There are no silently ignored predicates.
func (r CSVCaptureRequest) Validate() error {
	fail := func(field string) error { return fmt.Errorf("csv: invalid or unsupported %s", field) }
	s, ok := CSVSourceDefinition(r.Source)
	if !ok {
		return fail("source")
	}
	if r.Scope != s.Scope {
		return fail("scope")
	}
	if s.Scope == CSVCollection {
		if r.CollectionID == nil || *r.CollectionID == uuid.Nil {
			return fail("collectionId")
		}
	} else if r.CollectionID != nil {
		return fail("private collectionId")
	}
	if len(r.Columns) == 0 {
		return fail("columns")
	}
	seen := map[CSVColumn]bool{}
	for _, c := range r.Columns {
		if !slices.Contains(s.Columns, c) || seen[c] {
			return fail("columns")
		}
		seen[c] = true
	}
	seen = map[CSVColumn]bool{}
	for _, o := range r.Ordering {
		if !slices.Contains(s.OrderColumns, o.Column) || seen[o.Column] || (o.Direction != "asc" && o.Direction != "desc") {
			return fail("ordering")
		}
		seen[o.Column] = true
	}
	if r.Entities != nil {
		if !s.EntityFilter {
			return fail("entities")
		}
		f := r.Entities
		if f.OnlyWithPhoto && f.OnlyWithoutPhoto {
			return fail("photo filters")
		}
		if f.NegateTags && len(f.TagIDs) == 0 {
			return fail("negateTags")
		}
		if r.Source == "items" || r.Source == "dashboard-recent-items" || r.Source == "item-selector" {
			if f.IsLocation != nil && *f.IsLocation {
				return fail("isLocation")
			}
		}
		if r.Source == "locations" || r.Source == "location-selector" {
			if f.IsLocation != nil && !*f.IsLocation {
				return fail("isLocation")
			}
		}
		for _, ids := range [][]uuid.UUID{f.ParentIDs, f.ParentItemIDs, f.TagIDs} {
			if !csvValidIDs(ids) {
				return fail("entity reference IDs")
			}
		}
		fields := map[string]bool{}
		for _, f := range f.Fields {
			if f.Name == "" || fields[f.Name] {
				return fail("fields")
			}
			fields[f.Name] = true
		}
		if f.AssetID != "" {
			for _, c := range f.AssetID {
				if (c < '0' || c > '9') && c != '-' {
					return fail("assetId")
				}
			}
			if aid, ok := repo.ParseAssetID(f.AssetID); !ok || aid.Nil() {
				return fail("assetId")
			}
		}
	}
	if r.Text != nil && !s.TextFilter {
		return fail("text")
	}
	if r.Maintenance != nil {
		if !s.MaintenanceFilter || !slices.Contains([]string{"scheduled", "completed", "both"}, r.Maintenance.Status) {
			return fail("maintenance")
		}
	}
	if s.ContextKind != "" {
		if r.Hierarchy == nil || r.Hierarchy.Kind != s.ContextKind || r.Hierarchy.RootID == uuid.Nil {
			return fail("hierarchy")
		}
	} else if r.Hierarchy != nil {
		return fail("hierarchy")
	}
	if r.Hierarchy != nil && r.Hierarchy.Descendants && (s.Adapter == CSVAttachments || r.Source == "entity-maintenance") {
		return fail("hierarchy descendants")
	}
	if s.SelectorKind != "" {
		if r.Selector == nil || r.Selector.Kind != s.SelectorKind || r.Selector.Eligibility != "explicit-set" || r.Selector.EligibleIDs == nil || !csvValidIDs(r.Selector.EligibleIDs) || !csvValidIDs(r.Selector.ExcludedIDs) {
			return fail("selector")
		}
		for _, id := range r.Selector.ExcludedIDs {
			if !slices.Contains(r.Selector.EligibleIDs, id) {
				return fail("selector exclusions")
			}
		}
	} else if r.Selector != nil {
		return fail("selector")
	}
	if s.Recent {
		if r.Recent == nil || r.Recent.CapturedAt.IsZero() || r.Recent.RecordIDs == nil || !csvValidIDs(r.Recent.RecordIDs) {
			return fail("recent")
		}
	} else if r.Recent != nil {
		return fail("recent")
	}
	return nil
}
func csvValidIDs(ids []uuid.UUID) bool {
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
