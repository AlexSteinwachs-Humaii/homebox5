package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/schema/mixins"
)

// CsvExport is deliberately separate from portable ZIP backups/imports.
// Restrictive FKs keep the blob pointer until deletion cleanup is implemented.
type CsvExport struct{ ent.Schema }

func (CsvExport) Mixin() []ent.Mixin { return []ent.Mixin{mixins.BaseMixin{}} }
func (CsvExport) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("requester_id", uuid.UUID{}).Immutable(),
		field.UUID("group_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.Enum("scope").Values("collection", "private").Immutable(),
		field.String("source").NotEmpty().Immutable(),
		field.String("access_requirement").NotEmpty().Immutable(),
		field.JSON("request_snapshot", csvexports.Request{}).Immutable(),
		field.Int("retention_days").Default(csvexports.DefaultRetentionDays).Min(1).Max(csvexports.MaxRetentionDays).Immutable(),
		field.Enum("status").Values("pending", "running", "completed", "failed", "deleting").Default("pending"),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable(),
		field.String("artifact_path").Optional().NotEmpty().Sensitive(),
		field.Int64("size_bytes").Default(0).NonNegative(),
		field.String("error").MaxLen(1000).Optional(),
	}
}
func (CsvExport) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("requester", User.Type).Ref("csv_exports").Field("requester_id").Unique().Required().Immutable(),
		edge.From("group", Group.Type).Ref("csv_exports").Field("group_id").Unique().Immutable(),
	}
}
func (CsvExport) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "status"), index.Fields("requester_id", "scope"), index.Fields("expires_at"),
	}
}

func (CsvExport) Annotations() []entschema.Annotation {
	return []entschema.Annotation{entsql.Annotation{Checks: map[string]string{
		"csv_scope":     "((scope = 'collection' AND group_id IS NOT NULL) OR (scope = 'private' AND group_id IS NULL))",
		"csv_completed": "(status != 'completed' OR (completed_at IS NOT NULL AND expires_at IS NOT NULL AND expires_at > completed_at AND artifact_path IS NOT NULL AND artifact_path <> ''))",
	}}}
}
