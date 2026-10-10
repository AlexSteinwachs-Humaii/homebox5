package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
	_ "github.com/sysadminsmedia/homebox/backend/pkgs/cgofreesqlite"
)

func TestCSVAPIIndependentPrivateWorkflow(t *testing.T) {
	ctx := context.Background()
	db, err := ent.Open("sqlite3", "file:"+uuid.NewString()+"?mode=memory&cache=shared&_fk=1&_time_format=sqlite")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	require.NoError(t, db.Schema.Create(ctx))
	user, err := db.User.Create().SetName("private").SetEmail("private@test.invalid").Save(ctx)
	require.NoError(t, err)
	repos := repo.New(db, nil, config.Storage{ConnString: "file://" + t.TempDir(), PrefixPath: "csv-api"}, "", config.Thumbnail{})
	svc := services.NewCSVExportService(db, repos, nil)
	ctrl := &V1Controller{repo: repos, svc: &services.AllServices{CSVExports: svc}}
	router := chi.NewRouter()
	for _, op := range []struct{ method, path, name string }{
		{"POST", "/csv", "create"}, {"GET", "/csv", "history"}, {"GET", "/csv/{id}", "status"},
		{"GET", "/csv/{id}/download", "download"}, {"DELETE", "/csv/{id}", "delete"}, {"POST", "/csv/{id}/retry", "retry"},
	} {
		handler := ctrl.HandleCSV(true, op.name)
		router.MethodFunc(op.method, op.path, func(w http.ResponseWriter, r *http.Request) { require.NoError(t, handler(w, r)) })
	}
	call := func(method, path, body string, uid uuid.UUID) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		// Supplied tenant does not affect private scope, even without membership.
		req.Header.Set("X-Tenant", uuid.NewString())
		req = req.WithContext(services.SetUserCtx(ctx, &repo.UserOut{ID: uid}, "test"))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		require.Empty(t, w.Header().Get("Location"))
		return w
	}
	body := `{"source":"account-api-keys","scope":"account-private","columns":["name"]}`
	for _, tampered := range []string{
		strings.TrimSuffix(body, "}") + `,"blobKey":"other/path"}`,
		strings.Replace(body, `["name"]`, `["token"]`, 1),
		strings.TrimSuffix(body, "}") + `,"role":"owner"}`,
	} {
		require.Equal(t, 400, call("POST", "/csv", tampered, user.ID).Code)
	}
	created := call("POST", "/csv", body, user.ID)
	require.Equal(t, 202, created.Code)
	var row services.CSVArtifact
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &row))
	require.Equal(t, "queued", string(row.Status))
	require.NoError(t, svc.RunJob(ctx, row.ID))
	status := call("GET", "/csv/"+row.ID.String(), "", user.ID)
	require.Equal(t, 200, status.Code)
	require.NotContains(t, status.Body.String(), "blob")
	require.NotContains(t, status.Body.String(), "capture")
	require.Contains(t, status.Body.String(), "completedAt")
	downloaded := call("GET", "/csv/"+row.ID.String()+"/download", "", user.ID)
	require.Equal(t, 200, downloaded.Code)
	require.Equal(t, "name\n", downloaded.Body.String())
	// A disconnected/local-save client does not change the retained lifecycle.
	cancelReq := httptest.NewRequest("GET", "/csv/"+row.ID.String()+"/download", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", row.ID.String())
	cancelCtx := context.WithValue(services.SetUserCtx(ctx, &repo.UserOut{ID: user.ID}, "test"), chi.RouteCtxKey, routeCtx)
	cancelReq = cancelReq.WithContext(cancelCtx)
	require.ErrorIs(t, ctrl.HandleCSV(true, "download")(&csvDisconnectedWriter{ResponseRecorder: httptest.NewRecorder()}, cancelReq), errCSVDisconnected)
	retained, err := svc.Status(ctx, user.ID, nil, row.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", string(retained.Status))
	history := call("GET", "/csv", "", user.ID)
	require.Equal(t, 200, history.Code)
	require.Contains(t, history.Body.String(), row.ID.String())
	other, err := db.User.Create().SetName("other").SetEmail("other@test.invalid").Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 404, call("GET", "/csv/"+row.ID.String()+"/download", "", other.ID).Code)
	require.NotContains(t, call("GET", "/csv", "", other.ID).Body.String(), row.ID.String())
	require.Equal(t, 400, call("GET", "/csv/tampered", "", user.ID).Code)
	require.Equal(t, 409, call("POST", "/csv/"+row.ID.String()+"/retry", "", user.ID).Code)
	require.Equal(t, 202, call("DELETE", "/csv/"+row.ID.String(), "", user.ID).Code)
	require.Equal(t, 202, call("DELETE", "/csv/"+row.ID.String(), "", user.ID).Code)
	require.Equal(t, 409, call("GET", "/csv/"+row.ID.String()+"/download", "", user.ID).Code)
}

var errCSVDisconnected = errors.New("client disconnected")

type csvDisconnectedWriter struct{ *httptest.ResponseRecorder }

func (*csvDisconnectedWriter) Write([]byte) (int, error) { return 0, errCSVDisconnected }
