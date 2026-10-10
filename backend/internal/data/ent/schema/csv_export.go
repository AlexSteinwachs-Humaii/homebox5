package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/schema/mixins"
	"regexp"
)

// CSVExport is deliberately unrelated to Export (ZIP/import) and Attachment.
// References restrict parent deletion: cleanup must remove blobs before metadata.
type CSVExport struct{ ent.Schema }

func (CSVExport) Mixin() []ent.Mixin { return []ent.Mixin{mixins.BaseMixin{}} }
func (CSVExport) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}).Immutable(),
		field.UUID("group_id", uuid.UUID{}).Optional().Nillable().Immutable(),
		field.String("source").NotEmpty().Immutable(),
		field.JSON("capture", map[string]interface{}{}).Immutable(),
		field.Enum("scope").Values("collection", "account-private", "cross-collection").Immutable(),
		field.Enum("status").Values("queued", "running", "completed", "generation-failed", "storage-failed", "cleanup-pending").Default("queued"),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable(),
		// Server allocated before any upload, so crashes never lose the intended key.
		field.String("blob_key").Match(regexp.MustCompile(`^csv/[0-9a-f-]{36}/[0-9a-f-]{36}\.csv$`)).Unique().Immutable().Sensitive(),
		field.Int64("size_bytes").Default(0).NonNegative(),
		field.String("failure_code").Optional().MaxLen(100),
		field.String("failure_detail").Optional().MaxLen(1000).Sensitive(),
		field.Int("attempts").Default(0).NonNegative(),
		field.Time("retry_at").Optional().Nillable(),
		field.String("cleanup_error").Optional().MaxLen(1000).Sensitive(),
	}
}
func (CSVExport) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("csv_exports").Field("user_id").Unique().Required().Immutable(),
		edge.From("group", Group.Type).Ref("csv_exports").Field("group_id").Unique().Immutable().Annotations(entsql.OnDelete(entsql.NoAction)),
	}
}
func (CSVExport) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "scope", "created_at"),
		index.Fields("user_id", "scope", "created_at"),
		index.Fields("status", "expires_at"),
		index.Fields("status", "retry_at"),
	}
}
func (CSVExport) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Checks: map[string]string{
		"csv_scope_group":    "((scope = 'collection' AND group_id IS NOT NULL) OR (scope IN ('account-private', 'cross-collection') AND group_id IS NULL))",
		"csv_completion":     "(status <> 'completed' OR (completed_at IS NOT NULL AND expires_at IS NOT NULL AND expires_at > completed_at))",
		"csv_size":           "(size_bytes >= 0 AND attempts >= 0)",
		"csv_status":         "(status IN ('queued', 'running', 'completed', 'generation-failed', 'storage-failed', 'cleanup-pending'))",
		"csv_source":         "(length(source) > 0)",
		"csv_blob":           "(blob_key = 'csv/' || CAST(user_id AS TEXT) || '/' || CAST(id AS TEXT) || '.csv')",
		"csv_failure_code":   "(failure_code IS NULL OR length(failure_code) <= 100)",
		"csv_failure_detail": "(failure_detail IS NULL OR length(failure_detail) <= 1000)",
		"csv_cleanup_error":  "(cleanup_error IS NULL OR length(cleanup_error) <= 1000)",
	}}}
}
