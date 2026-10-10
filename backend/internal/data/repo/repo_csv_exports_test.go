package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/migrations"
	_ "github.com/sysadminsmedia/homebox/backend/internal/data/migrations/postgres"
	_ "github.com/sysadminsmedia/homebox/backend/internal/data/migrations/sqlite3"
)

// PostgreSQL tests create a temporary database (historical migrations explicitly
// reference public). CSV_TEST_POSTGRES_URL needs CREATEDB, but is never reset.
func csvTestDB(t *testing.T, driver string) (*sql.DB, *ent.Client) {
	t.Helper()
	var db *sql.DB
	var err error
	if driver == "sqlite3" {
		db, err = sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1&_time_format=sqlite", uuid.New()))
	} else {
		dsn := os.Getenv("CSV_TEST_POSTGRES_URL")
		if dsn == "" {
			t.Skip("set CSV_TEST_POSTGRES_URL to exercise PostgreSQL")
		}
		admin, e := sql.Open("pgx", dsn)
		require.NoError(t, e)
		name := "csv_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		_, e = admin.Exec("CREATE DATABASE " + name)
		require.NoError(t, e)
		t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE " + name); _ = admin.Close() })
		u, e := url.Parse(dsn)
		require.NoError(t, e)
		u.Path = "/" + name
		db, err = sql.Open("pgx", u.String())
	}
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	d := dialect.SQLite
	if driver == "postgres" {
		d = dialect.Postgres
	}
	return db, ent.NewClient(ent.Driver(entsql.OpenDB(d, db)))
}

func TestCSVSchemaAndPersistence(t *testing.T) {
	for _, driver := range []string{"sqlite3", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			db, client := csvTestDB(t, driver)
			require.NoError(t, client.Schema.Create(context.Background()))
			verifyCSVPersistence(t, db, client, driver)
		})
	}
}

func verifyCSVPersistence(t *testing.T, db *sql.DB, client *ent.Client, driver string) {
	t.Helper()
	ctx := context.Background()
	r := &CSVExportRepository{client}
	u, err := client.User.Create().SetName("CSV owner").SetEmail(uuid.NewString() + "@example.test").Save(ctx)
	require.NoError(t, err)
	other, err := client.User.Create().SetName("Other").SetEmail(uuid.NewString() + "@example.test").Save(ctx)
	require.NoError(t, err)
	g, err := client.Group.Create().SetName("CSV group").Save(ctx)
	require.NoError(t, err)
	otherG, err := client.Group.Create().SetName("Other group").Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 30, u.CsvRetentionDays)
	require.Equal(t, 30, g.CsvRetentionDays)
	for _, days := range []int{0, -1, MaxCSVRetentionDays + 1, int(^uint(0) >> 1)} {
		require.Error(t, r.SetUserRetention(ctx, u.ID, days))
		require.Error(t, r.SetGroupRetention(ctx, g.ID, days))
		require.Error(t, client.User.UpdateOneID(u.ID).SetCsvRetentionDays(days).Exec(ctx))
		require.Error(t, client.Group.UpdateOneID(g.ID).SetCsvRetentionDays(days).Exec(ctx))
	}
	for _, badJSON := range []string{`1.5`, `0`, `-1`, `9223372036854775808`} {
		var days int
		err := json.Unmarshal([]byte(badJSON), &days)
		if err == nil {
			err = r.SetUserRetention(ctx, u.ID, days)
		}
		require.Error(t, err)
	}
	require.NoError(t, r.SetUserRetention(ctx, u.ID, MaxCSVRetentionDays))
	require.NoError(t, r.SetGroupRetention(ctx, g.ID, 1))
	days, err := r.UserRetention(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, MaxCSVRetentionDays, days)
	days, err = r.GroupRetention(ctx, g.ID)
	require.NoError(t, err)
	require.Equal(t, 1, days)
	// Whole-day validation is also enforced for SQLite's dynamically typed SQL.
	if driver == "sqlite3" {
		_, err = db.Exec("UPDATE users SET csv_retention_days=1.5 WHERE id=?", u.ID)
		require.Error(t, err)
	}

	capture := map[string]interface{}{"source": "items", "columns": []interface{}{"name"}}
	collection, err := r.Create(ctx, u.ID, &g.ID, csvexport.ScopeCollection, "items", capture)
	require.NoError(t, err)
	private, err := r.Create(ctx, u.ID, nil, csvexport.ScopeAccountPrivate, "account-api-keys", map[string]interface{}{"source": "account-api-keys"})
	require.NoError(t, err)
	cross, err := r.Create(ctx, u.ID, nil, csvexport.ScopeCrossCollection, "account-collections", map[string]interface{}{"source": "account-collections"})
	require.NoError(t, err)
	require.Nil(t, private.GroupID)
	require.Nil(t, cross.GroupID)
	require.Equal(t, fmt.Sprintf("csv/%s/%s.csv", u.ID, collection.ID), collection.BlobKey)
	require.Equal(t, capture, collection.Capture)
	rows, err := r.ListByGroup(ctx, g.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, collection.ID, rows[0].ID)
	rows, err = r.ListPrivate(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	rows, err = r.ListPrivate(ctx, other.ID)
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = r.GetByGroup(ctx, g.ID, private.ID)
	require.True(t, ent.IsNotFound(err))
	_, err = r.GetByGroup(ctx, otherG.ID, collection.ID)
	require.True(t, ent.IsNotFound(err))
	_, err = r.GetPrivate(ctx, other.ID, private.ID)
	require.True(t, ent.IsNotFound(err))
	_, err = r.GetPrivate(ctx, u.ID, collection.ID)
	require.True(t, ent.IsNotFound(err))
	_, err = r.Create(ctx, u.ID, &g.ID, csvexport.ScopeAccountPrivate, "items", capture)
	require.Error(t, err)
	_, err = r.Create(ctx, u.ID, nil, csvexport.ScopeCollection, "items", capture)
	require.Error(t, err)
	// Direct Ent writes also receive relational checks, independent of the repo.
	badID := uuid.New()
	_, err = client.CSVExport.Create().SetID(badID).SetUserID(u.ID).SetGroupID(g.ID).SetScope(csvexport.ScopeAccountPrivate).SetSource("items").SetCapture(capture).SetBlobKey(fmt.Sprintf("csv/%s/%s.csv", u.ID, badID)).Save(ctx)
	require.Error(t, err)
	require.Error(t, client.CSVExport.UpdateOneID(collection.ID).SetStatus(csvexport.StatusCompleted).Exec(ctx))
	require.Error(t, client.CSVExport.Create().SetUserID(u.ID).SetScope(csvexport.ScopeAccountPrivate).SetSource("items").SetCapture(capture).SetBlobKey("arbitrary.zip").Exec(ctx))
	now := time.Now().UTC().Truncate(time.Second)
	require.Error(t, client.CSVExport.UpdateOneID(collection.ID).SetStatus(csvexport.StatusCompleted).SetCompletedAt(now).SetExpiresAt(now).Exec(ctx))
	require.NoError(t, client.CSVExport.UpdateOneID(collection.ID).SetStatus(csvexport.StatusCompleted).SetCompletedAt(now).SetExpiresAt(now.Add(time.Hour)).SetSizeBytes(12).Exec(ctx))
	require.NoError(t, client.CSVExport.UpdateOneID(cross.ID).SetStatus(csvexport.StatusStorageFailed).SetFailureCode("blob-upload").SetFailureDetail("recoverable internal error").SetRetryAt(now).SetAttempts(2).Exec(ctx))
	failed, err := r.GetPrivate(ctx, u.ID, cross.ID)
	require.NoError(t, err)
	require.Equal(t, csvexport.StatusStorageFailed, failed.Status)
	require.Equal(t, "blob-upload", failed.FailureCode)
	require.Equal(t, 2, failed.Attempts)
	require.NoError(t, client.CSVExport.UpdateOneID(private.ID).SetStatus(csvexport.StatusCleanupPending).SetCleanupError("retry delete").SetRetryAt(now.Add(time.Hour)).SetAttempts(1).Exec(ctx))
	rows, err = r.CleanupCandidates(ctx, now)
	require.NoError(t, err)
	require.Empty(t, rows)
	rows, err = r.CleanupCandidates(ctx, now.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	fetched, err := r.GetPrivate(ctx, u.ID, private.ID)
	require.NoError(t, err)
	require.Equal(t, "retry delete", fetched.CleanupError)
	require.Equal(t, 1, fetched.Attempts)
	encoded, err := json.Marshal(fetched)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "csv/")
	require.NotContains(t, string(encoded), "retry delete")
	require.Error(t, client.User.DeleteOneID(u.ID).Exec(ctx))
	require.Error(t, client.Group.DeleteOneID(g.ID).Exec(ctx))
	count, err := client.Export.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	// Both migration and generated schemas support tenant history, expiry sweeps
	// and recoverable-worker retry scans with the same named composite indexes.
	indexQuery := "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='csv_exports'"
	if driver == "postgres" {
		indexQuery = "SELECT indexname FROM pg_indexes WHERE schemaname='public' AND tablename='csv_exports'"
	}
	indexes, err := db.Query(indexQuery)
	require.NoError(t, err)
	var names []string
	for indexes.Next() {
		var name string
		require.NoError(t, indexes.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, indexes.Err())
	require.NoError(t, indexes.Close())
	for _, name := range []string{"csvexport_group_id_scope_created_at", "csvexport_user_id_scope_created_at", "csvexport_status_expires_at", "csvexport_status_retry_at"} {
		require.Contains(t, names, name)
	}
}

func TestCSVMigrationUpgrade(t *testing.T) {
	for _, driver := range []string{"sqlite3", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			ctx := context.Background()
			db, client := csvTestDB(t, driver)
			files, err := migrations.Migrations(driver)
			require.NoError(t, err)
			goose.SetBaseFS(files)
			require.NoError(t, goose.SetDialect(driver))
			previous := int64(20260512130000)
			if driver == "postgres" {
				previous++
			}
			err = goose.UpToContext(ctx, db, driver, previous)
			require.NoError(t, err)
			uid, gid, zipID, importID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			mark := "?"
			if driver == "postgres" {
				mark = "$1"
			}
			_, err = db.Exec("INSERT INTO users(id,created_at,updated_at,name,email,password) VALUES ("+mark+",CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,'existing','existing@example.test','hash')", uid)
			require.NoError(t, err)
			_, err = db.Exec("INSERT INTO groups(id,created_at,updated_at,name,currency) VALUES ("+mark+",CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,'existing','usd')", gid)
			require.NoError(t, err)
			m2 := "?"
			if driver == "postgres" {
				m2 = "$2"
			}
			for _, row := range []struct {
				id   uuid.UUID
				kind string
			}{{zipID, "export"}, {importID, "import"}} {
				_, err = db.Exec("INSERT INTO exports(id,created_at,updated_at,kind,status,progress,artifact_path,size_bytes,group_id) VALUES ("+mark+",CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,'"+row.kind+"','completed',100,'original.zip',42,"+m2+")", row.id, gid)
				require.NoError(t, err)
			}
			err = goose.UpContext(ctx, db, driver)
			require.NoError(t, err)
			u, err := client.User.Get(ctx, uid)
			require.NoError(t, err)
			require.Equal(t, 30, u.CsvRetentionDays)
			g, err := client.Group.Get(ctx, gid)
			require.NoError(t, err)
			require.Equal(t, 30, g.CsvRetentionDays)
			for _, id := range []uuid.UUID{zipID, importID} {
				z, err := client.Export.Get(ctx, id)
				require.NoError(t, err)
				require.Equal(t, "original.zip", z.ArtifactPath)
				require.EqualValues(t, 42, z.SizeBytes)
			}
			// Verify all behavior on upgraded tables, not just fresh Ent schemas.
			require.NoError(t, client.Export.DeleteOneID(zipID).Exec(ctx))
			require.NoError(t, client.Export.DeleteOneID(importID).Exec(ctx))
			verifyCSVPersistence(t, db, client, driver)
		})
	}
}
