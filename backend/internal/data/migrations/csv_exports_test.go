package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/sysadminsmedia/homebox/backend/pkgs/cgofreesqlite"
)

// Uses the deployed SQL (not Ent Schema.Create). PostgreSQL can be exercised in
// CI with CSV_EXPORT_TEST_POSTGRES_DSN; each run rolls back its isolated schema.
func TestCsvExportMigration(t *testing.T) {
	for _, dialect := range []string{"sqlite3", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			driver, dsn := dialect, "file:csv_migration_"+uuid.NewString()+"?mode=memory&_fk=1&_time_format=sqlite"
			if dialect == "postgres" {
				driver = "pgx"
				dsn = os.Getenv("CSV_EXPORT_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("CSV_EXPORT_TEST_POSTGRES_DSN not configured")
				}
			}
			db, err := sql.Open(driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			exec := func(query string) {
				t.Helper()
				if _, err := tx.Exec(query); err != nil {
					t.Fatalf("%s: %v", query, err)
				}
			}
			if dialect == "postgres" {
				schema := "csv_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
				exec("CREATE SCHEMA " + schema)
				exec("SET LOCAL search_path TO " + schema)
			}
			exec("CREATE TABLE users (id uuid PRIMARY KEY); CREATE TABLE groups (id uuid PRIMARY KEY); CREATE TABLE exports (id uuid PRIMARY KEY, kind text, status text);")
			uid, gid := uuid.NewString(), uuid.NewString()
			exec(fmt.Sprintf("INSERT INTO users(id) VALUES('%s'); INSERT INTO groups(id) VALUES('%s'); INSERT INTO exports VALUES('%s','export','completed');", uid, gid, uuid.NewString()))
			fs, err := Migrations(dialect)
			if err != nil {
				t.Fatal(err)
			}
			data, err := fs.ReadFile(dialect + "/20261009000000_add_csv_exports.sql")
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.Split(string(data), "-- +goose Down")
			exec(strings.TrimPrefix(parts[0], "-- +goose Up"))
			var days int
			if err = tx.QueryRow("SELECT csv_export_retention_days FROM users").Scan(&days); err != nil || days != 30 {
				t.Fatalf("private default: %d %v", days, err)
			}
			if err = tx.QueryRow("SELECT csv_export_retention_days FROM groups").Scan(&days); err != nil || days != 30 {
				t.Fatalf("collection default: %d %v", days, err)
			}
			row := fmt.Sprintf("INSERT INTO csv_exports(id,created_at,updated_at,requester_id,scope,source,access_requirement,request_snapshot) VALUES('%s','2026-10-09 12:00:00+00:00','2026-10-09 12:00:00+00:00','%s','private','api-keys','requester','{}')", uuid.NewString(), uid)
			exec(row)
			// Savepoints allow expected constraint failures without aborting PG's transaction.
			rejects := []string{"UPDATE users SET csv_export_retention_days=0", "UPDATE groups SET csv_export_retention_days=-1", "UPDATE csv_exports SET retention_days=0", "UPDATE csv_exports SET status='completed'", fmt.Sprintf("UPDATE csv_exports SET group_id='%s'", gid), fmt.Sprintf("DELETE FROM users WHERE id='%s'", uid)}
			if dialect == "sqlite3" {
				rejects = append(rejects, "UPDATE users SET csv_export_retention_days=1.5", "UPDATE groups SET csv_export_retention_days=2.5", "UPDATE csv_exports SET retention_days=3.5")
			}
			for i, query := range rejects {
				exec(fmt.Sprintf("SAVEPOINT invalid_%d", i))
				if _, err = tx.Exec(query); err == nil {
					t.Fatalf("accepted invalid persisted contract: %s", query)
				}
				exec(fmt.Sprintf("ROLLBACK TO SAVEPOINT invalid_%d", i))
			}
			var count int
			if err = tx.QueryRow("SELECT count(*) FROM exports WHERE kind='export' AND status='completed'").Scan(&count); err != nil || count != 1 {
				t.Fatal("ZIP lifecycle changed")
			}
			exec(parts[1])
			if err = tx.QueryRow("SELECT count(*) FROM exports").Scan(&count); err != nil || count != 1 {
				t.Fatal("rollback changed ZIP lifecycle")
			}
		})
	}
}
