package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/reporting/eventbus"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/export"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
)

func TestSevenDayZIPCleanupDoesNotTouchCSV(t *testing.T) {
	ctx := context.Background()
	db, err := ent.Open("sqlite3", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1&_time_format=sqlite", uuid.New()))
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Schema.Create(ctx))
	dir := t.TempDir()
	repos := repo.New(db, eventbus.New(), config.Storage{ConnString: "file://" + dir}, "mem://{{ .Topic }}", config.Thumbnail{})
	g, err := db.Group.Create().SetName("ZIP regression").Save(ctx)
	require.NoError(t, err)
	u, err := db.User.Create().SetName("CSV owner").SetEmail("csv@example.test").Save(ctx)
	require.NoError(t, err)
	now := time.Now()
	var freshID uuid.UUID
	for _, kind := range []export.Kind{export.KindExport, export.KindImport} {
		for _, days := range []int{8, 6} {
			key := fmt.Sprintf("%s-%d.zip", kind, days)
			require.NoError(t, os.WriteFile(filepath.Join(dir, key), []byte("ZIP fixture"), 0600))
			row, err := db.Export.Create().SetGroupID(g.ID).SetKind(kind).SetCreatedAt(now.Add(-time.Duration(days) * 24 * time.Hour)).SetArtifactPath(key).Save(ctx)
			require.NoError(t, err)
			if days == 6 {
				freshID = row.ID
			}
		}
	}
	csv, err := repos.CSVExports.Create(ctx, u.ID, nil, csvexport.ScopeAccountPrivate, "account-api-keys", map[string]interface{}{"source": "account-api-keys"})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, csv.BlobKey)), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, csv.BlobKey), []byte("CSV fixture"), 0600))
	purgeStaleExports(ctx, &app{repos: repos})
	rows, err := db.Export.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	_, err = db.Export.Get(ctx, freshID)
	require.NoError(t, err)
	for _, kind := range []export.Kind{export.KindExport, export.KindImport} {
		_, err = os.Stat(filepath.Join(dir, fmt.Sprintf("%s-8.zip", kind)))
		require.True(t, os.IsNotExist(err))
		_, err = os.Stat(filepath.Join(dir, fmt.Sprintf("%s-6.zip", kind)))
		require.NoError(t, err)
	}
	_, err = db.CSVExport.Get(ctx, csv.ID)
	require.NoError(t, err)
	bytes, err := os.ReadFile(filepath.Join(dir, csv.BlobKey))
	require.NoError(t, err)
	require.Equal(t, "CSV fixture", string(bytes))
}
