// Package csvexports defines CSV contracts only. Registry membership is not
// authorization: adapters must check current source access on every operation.
package csvexports

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

const DefaultRetentionDays = 30

// MaxRetentionDays is the largest whole-day duration representable by Go.
const MaxRetentionDays = int((1<<63 - 1) / int64(24*time.Hour))

type RetentionSettings struct {
	Days int `json:"days"`
}

func (s RetentionSettings) Validate() error {
	if s.Days < 1 || s.Days > MaxRetentionDays {
		return errors.New("retention must be positive whole days within timestamp arithmetic limits")
	}
	return nil
}

type Column struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type Sort struct {
	Column    string `json:"column"`
	Direction string `json:"direction"`
}
type FieldFilter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// No pagination: all matching records are required. Scope identities are derived
// from auth/tenant context, never carried in this request.
type Filters struct {
	FieldName        string        `json:"fieldName,omitempty"`
	Search           string        `json:"search,omitempty"`
	IsLocation       *bool         `json:"isLocation,omitempty"`
	ParentIDs        []uuid.UUID   `json:"parentIds,omitempty"`
	TagIDs           []uuid.UUID   `json:"tagIds,omitempty"`
	ParentItemIDs    []uuid.UUID   `json:"parentItemIds,omitempty"`
	Fields           []FieldFilter `json:"fields,omitempty"`
	AssetID          *int          `json:"assetId,omitempty"`
	NegateTags       *bool         `json:"negateTags,omitempty"`
	OnlyWithoutPhoto *bool         `json:"onlyWithoutPhoto,omitempty"`
	OnlyWithPhoto    *bool         `json:"onlyWithPhoto,omitempty"`
	IncludeArchived  *bool         `json:"includeArchived,omitempty"`
	FilterChildren   *bool         `json:"filterChildren,omitempty"`
	ShowItems        *bool         `json:"showItems,omitempty"`
	EntityID         *uuid.UUID    `json:"entityId,omitempty"`
	TemplateID       *uuid.UUID    `json:"templateId,omitempty"`
	Status           string        `json:"status,omitempty"`
	AttachmentType   string        `json:"attachmentType,omitempty"`
}
type Boundary struct {
	Kind string `json:"kind"`
	// Recent limits are intentional source boundaries, never a page size.
	Limit      int         `json:"limit,omitempty"`
	RecordIDs  []uuid.UUID `json:"recordIds,omitempty"`
	ExcludeIDs []uuid.UUID `json:"excludeIds,omitempty"`
}

// Bounded displayed records for unsaved/private data, not arbitrary object maps.
// Token is only accepted for a visible invitation owned by the current user;
// adapters must revalidate its collection, never reconstruct plaintext hashes.
type SnapshotRow struct {
	Manufacturer     string     `json:"manufacturer,omitempty"`
	ModelNumber      string     `json:"modelNumber,omitempty"`
	SearchEngineName string     `json:"searchEngineName,omitempty"`
	ID               *uuid.UUID `json:"id,omitempty"`
	Name             string     `json:"name,omitempty"`
	Type             string     `json:"type,omitempty"`
	Value            string     `json:"value,omitempty"`
	MimeType         string     `json:"mimeType,omitempty"`
	SizeBytes        *int64     `json:"sizeBytes,omitempty"`
	Token            string     `json:"token,omitempty"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	RemainingUses    *int       `json:"remainingUses,omitempty"`
}
type Request struct {
	Source   string        `json:"source"`
	Scope    string        `json:"scope"` // collection or private; must match registry
	Filters  Filters       `json:"filters"`
	Boundary Boundary      `json:"boundary"`
	Sort     []Sort        `json:"sort"`
	Columns  []Column      `json:"columns"`
	Snapshot []SnapshotRow `json:"snapshot,omitempty"`
}

// DecodeRequest is the only request decoder for future CSV endpoints. Rejects
// unknown fields recursively (SQL, keys, requester/permission claims included).
func DecodeRequest(r io.Reader) (Request, error) {
	var req Request
	data, err := io.ReadAll(io.LimitReader(r, 1<<20+1))
	if err != nil {
		return req, err
	}
	if len(data) > 1<<20 {
		return req, errors.New("CSV request too large")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&req); err != nil {
		return req, err
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return req, errors.New("expected one JSON request")
	}
	return req, req.Validate()
}
func (r Request) Validate() error {
	s, ok := Lookup(r.Source)
	if !ok {
		return errors.New("unsupported CSV source")
	}
	if r.Scope != s.Scope {
		return errors.New("source retention scope mismatch")
	}
	if !contains(s.Boundaries, r.Boundary.Kind) {
		return errors.New("unsupported source boundary")
	}
	if len(r.Columns) == 0 || len(r.Columns) > len(s.Columns) {
		return errors.New("ordered data columns required")
	}
	seen := map[string]bool{}
	for _, c := range r.Columns {
		if !contains(s.Columns, c.ID) || seen[c.ID] || len(c.Label) > 256 {
			return errors.New("invalid, hidden or duplicate column")
		}
		seen[c.ID] = true
	}
	if len(r.Sort) > len(s.Columns) {
		return errors.New("too many sort columns")
	}
	seen = map[string]bool{}
	for _, srt := range r.Sort {
		if !contains(s.Columns, srt.Column) || seen[srt.Column] || (srt.Direction != "asc" && srt.Direction != "desc") {
			return errors.New("invalid sort")
		}
		seen[srt.Column] = true
	}
	b, _ := json.Marshal(r.Filters)
	var filters map[string]any
	_ = json.Unmarshal(b, &filters)
	for f := range filters {
		if !contains(s.Filters, f) {
			return fmt.Errorf("unsupported %s filter", f)
		}
	}
	if len(r.Filters.Search) > 1000 || len(r.Filters.Fields) > 100 {
		return errors.New("filters too large")
	}
	if r.Filters.OnlyWithPhoto != nil && *r.Filters.OnlyWithPhoto && r.Filters.OnlyWithoutPhoto != nil && *r.Filters.OnlyWithoutPhoto {
		return errors.New("conflicting photo filters")
	}
	if r.Boundary.Kind == "recent" {
		if r.Boundary.Limit < 1 || r.Boundary.Limit > 1000 {
			return errors.New("recent boundary requires an explicit limit")
		}
	} else if r.Boundary.Limit != 0 {
		return errors.New("limit is not pagination")
	}
	if r.Boundary.Kind != "explicit" && len(r.Boundary.RecordIDs) > 0 {
		return errors.New("IDs require explicit boundary")
	}
	if r.Boundary.Kind != "eligible" && r.Boundary.Kind != "explicit" && len(r.Boundary.ExcludeIDs) > 0 {
		return errors.New("exclusions require eligible or explicit boundary")
	}
	for _, ids := range [][]uuid.UUID{r.Boundary.RecordIDs, r.Boundary.ExcludeIDs, r.Filters.ParentIDs, r.Filters.TagIDs, r.Filters.ParentItemIDs} {
		if len(ids) > 10000 {
			return errors.New("too many record IDs")
		}
		seenIDs := map[uuid.UUID]bool{}
		for _, id := range ids {
			if id == uuid.Nil || seenIDs[id] {
				return errors.New("invalid or duplicate record ID")
			}
			seenIDs[id] = true
		}
	}
	if r.Filters.EntityID != nil && *r.Filters.EntityID == uuid.Nil || r.Filters.TemplateID != nil && *r.Filters.TemplateID == uuid.Nil {
		return errors.New("invalid context ID")
	}
	if r.Boundary.Kind == "children" && r.Filters.EntityID == nil && r.Filters.TemplateID == nil {
		return errors.New("children require context")
	}
	if len(r.Snapshot) > 1000 || r.Boundary.Kind != "snapshot" && len(r.Snapshot) > 0 {
		return errors.New("invalid snapshot")
	}
	for _, row := range r.Snapshot {
		b, _ := json.Marshal(row)
		var fields map[string]any
		_ = json.Unmarshal(b, &fields)
		for f := range fields {
			if !contains(s.Columns, f) {
				return fmt.Errorf("unsupported snapshot field %s", f)
			}
		}
		if row.SizeBytes != nil && *row.SizeBytes < 0 {
			return errors.New("invalid snapshot size")
		}
	}
	return nil
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// Artifact is safe history metadata: no blob keys or credential URLs.
type Artifact struct {
	ID                uuid.UUID  `json:"id"`
	RequesterID       uuid.UUID  `json:"requesterId"`
	GroupID           *uuid.UUID `json:"groupId,omitempty"`
	Scope             string     `json:"scope"`
	Source            string     `json:"source"`
	AccessRequirement string     `json:"accessRequirement"`
	Request           Request    `json:"request"`
	RetentionDays     int        `json:"retentionDays"`
	CreatedAt         time.Time  `json:"createdAt"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
	ExpiresAt         *time.Time `json:"expiresAt,omitempty"`
	Status            string     `json:"status"`
	SizeBytes         int64      `json:"sizeBytes"`
	Error             string     `json:"error,omitempty"`
}
