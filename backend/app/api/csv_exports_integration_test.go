package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/rs/zerolog"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/reporting/eventbus"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/usergroup"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
	"github.com/sysadminsmedia/homebox/backend/internal/web/mid"
	_ "github.com/sysadminsmedia/homebox/backend/pkgs/cgofreesqlite"
	"github.com/sysadminsmedia/homebox/backend/pkgs/hasher"
)

func TestCSVAccountRoutesWithoutCollection(t *testing.T) {
	ctx := context.Background()
	db, e := ent.Open("sqlite3", "file:"+uuid.NewString()+"?mode=memory&cache=shared&_fk=1&_time_format=sqlite")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.Schema.Create(ctx); e != nil {
		t.Fatal(e)
	}
	bus := eventbus.New()
	repos := repo.New(db, bus, config.Storage{PrefixPath: "/", ConnString: "file://" + t.TempDir()}, "mem://{{ .Topic }}", config.Thumbnail{})
	svc := services.New(repos)
	g, e := repos.Groups.GroupCreate(ctx, "temporary", uuid.Nil)
	if e != nil {
		t.Fatal(e)
	}
	password, e := hasher.HashPassword("password")
	if e != nil {
		t.Fatal(e)
	}
	u, e := repos.Users.Create(ctx, repo.UserCreate{Name: "CSV account", Email: "csv@example.com", Password: &password, DefaultGroupID: g.ID})
	if e != nil {
		t.Fatal(e)
	}
	token, e := svc.User.Login(ctx, u.Email, "password", false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.UserGroup.Delete().Where(usergroup.UserID(u.ID)).Exec(ctx); e != nil {
		t.Fatal(e)
	}
	if e = db.User.UpdateOneID(u.ID).ClearDefaultGroupID().Exec(ctx); e != nil {
		t.Fatal(e)
	}
	a := &app{repos: repos, services: svc, bus: bus, conf: &config.Config{}}
	router := chi.NewRouter()
	a.mountRoutes(router, errchain.New(mid.Errors(zerolog.Nop())), repos)
	request := func(method, url, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, url, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token.Raw)
		// A stale/arbitrary tenant must not affect account-private endpoints.
		r.Header.Set("X-Tenant", uuid.NewString())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	base := "/api/v1/user/csv-exports"
	w := request("GET", base+"/retention", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "30") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("PUT", base+"/retention", `{"days":1}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", base, `{"source":"draft-fields","scope":"private","boundary":{"kind":"snapshot"},"columns":[{"id":"name"}],"snapshot":[{"name":"Unicode 雪"}]}`)
	if w.Code != 202 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	var artifact csvexports.Artifact
	if e = json.Unmarshal(w.Body.Bytes(), &artifact); e != nil {
		t.Fatal(e)
	}
	if artifact.GroupID != nil || artifact.Status != "pending" {
		t.Fatal(artifact)
	}
	if e := svc.CSVExports.Sweep(ctx); e != nil {
		t.Fatal(e)
	}
	url := base + "/" + artifact.ID.String()
	// Cross-user private requests are rejected even with a valid session.
	other, e := repos.Users.Create(ctx, repo.UserCreate{Name: "Other", Email: "other@example.com", Password: &password, DefaultGroupID: g.ID})
	if e != nil {
		t.Fatal(e)
	}
	otherToken, e := svc.User.Login(ctx, other.Email, "password", false)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", url+"/download", nil)
	r.Header.Set("Authorization", "Bearer "+otherToken.Raw)
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, r)
	if denied.Code != 404 {
		t.Fatal("cross-user access", denied.Code, denied.Body.String())
	}
	r = httptest.NewRequest("PUT", "/api/v1/group/csv-exports/retention", strings.NewReader(`{"days":5}`))
	r.Header.Set("Authorization", "Bearer "+otherToken.Raw)
	r.Header.Set("X-Tenant", g.ID.String())
	denied = httptest.NewRecorder()
	router.ServeHTTP(denied, r)
	if denied.Code != 403 {
		t.Fatal("member changed policy", denied.Code, denied.Body.String())
	}
	w = request("GET", url+"/download", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Unicode 雪") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	if strings.Contains(request("GET", base, "").Body.String(), "artifact_path") {
		t.Fatal("blob key exposed")
	}
	w = request("GET", url+"/download", "")
	canonical := w.Body.String()
	w = request("GET", url+"/download", "")
	if w.Body.String() != canonical {
		t.Fatal("different bytes")
	}
	row, e := db.CsvExport.Get(ctx, artifact.ID)
	if e != nil {
		t.Fatal(e)
	}
	// The API rejects expiry before writing CSV bytes even while the blob exists.
	if e = db.CsvExport.UpdateOneID(row.ID).SetCompletedAt(time.Now().Add(-48 * time.Hour)).SetExpiresAt(time.Now().Add(-time.Hour)).Exec(ctx); e != nil {
		t.Fatal(e)
	}
	w = request("GET", url+"/download", "")
	if w.Code != 410 || strings.Contains(w.Body.String(), "Unicode 雪") {
		t.Fatal("expired bytes", w.Code, w.Body.String())
	}
	w = request("DELETE", url, "")
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("DELETE", url, "")
	if w.Code != 204 {
		t.Fatal("delete not idempotent", w.Code, w.Body.String())
	}
}
