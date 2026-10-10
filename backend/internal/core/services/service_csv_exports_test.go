package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/usergroup"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
)

type csvMemoryStore struct {
	objects              map[string][]byte
	puts, opens, deletes int
	putErr, deleteErr    bool
	beforePut            func()
}

func (s *csvMemoryStore) Put(_ context.Context, key string, b []byte) error {
	s.puts++
	if s.beforePut != nil {
		s.beforePut()
	}
	s.objects[key] = bytes.Clone(b)
	if s.putErr {
		return errors.New("storage secret path")
	}
	return nil
}
func (s *csvMemoryStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	s.opens++
	b, ok := s.objects[key]
	if !ok {
		return nil, errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (s *csvMemoryStore) Delete(_ context.Context, key string) error {
	s.deletes++
	if s.deleteErr {
		return errors.New("cleanup failed")
	}
	delete(s.objects, key)
	return nil
}
func csvTestService() (*CSVExportService, *csvMemoryStore) {
	st := &csvMemoryStore{objects: map[string][]byte{}}
	return &CSVExportService{repos: tRepos, store: st}, st
}
func csvDraftRequest() csvexports.Request {
	return csvexports.Request{Source: "draft-fields", Scope: "private", Boundary: csvexports.Boundary{Kind: "snapshot"}, Columns: []csvexports.Column{{ID: "name"}, {ID: "value"}}, Snapshot: []csvexports.SnapshotRow{{Name: "雪", Value: "=formula"}}}
}

func TestCSVCanonicalRetentionExpiryAndCleanupRecovery(t *testing.T) {
	ctx := context.Background()
	s, st := csvTestService()
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	if e := tRepos.CsvExports.SetRetention(ctx, tUser.ID, nil, csvexports.RetentionSettings{Days: 2}); e != nil {
		t.Fatal(e)
	}
	defer tRepos.CsvExports.SetRetention(ctx, tUser.ID, nil, csvexports.RetentionSettings{Days: 30})
	out, e := s.Create(ctx, tUser.ID, nil, csvDraftRequest())
	if e != nil {
		t.Fatal(e)
	}
	if out.Status != "pending" || out.CompletedAt != nil {
		t.Fatal(out)
	}
	defer tClient.CsvExport.DeleteOneID(out.ID).Exec(ctx)
	if e := tRepos.CsvExports.SetRetention(ctx, tUser.ID, nil, csvexports.RetentionSettings{Days: 9}); e != nil {
		t.Fatal(e)
	}
	now = now.Add(3 * time.Hour)
	row, _ := tClient.CsvExport.Get(ctx, out.ID)
	st.beforePut = func() {
		running, _ := s.Get(ctx, tUser.ID, nil, out.ID)
		if running.Status != "running" {
			t.Error(running)
		}
	}
	if e := s.run(ctx, row); e != nil {
		t.Fatal(e)
	}
	ready, e := s.Get(ctx, tUser.ID, nil, out.ID)
	if e != nil {
		t.Fatal(e)
	}
	if ready.Status != "completed" || ready.RetentionDays != 2 || !ready.CompletedAt.Equal(now) || !ready.ExpiresAt.Equal(now.Add(48*time.Hour)) {
		t.Fatal(ready)
	}
	var first []byte
	for i := 0; i < 2; i++ {
		reader, _, e := s.Download(ctx, tUser.ID, nil, out.ID)
		if e != nil {
			t.Fatal(e)
		}
		b, _ := io.ReadAll(reader)
		reader.Close()
		if i == 0 {
			first = b
		} else if !bytes.Equal(first, b) {
			t.Fatal("different canonical bytes")
		}
	}
	if !strings.Contains(string(first), "'=formula") || st.puts != 1 {
		t.Fatal(string(first), st.puts)
	}
	// A duplicate delivery cannot overwrite a completed artifact.
	if e := s.run(ctx, row); e != nil {
		t.Fatal(e)
	}
	if st.puts != 1 {
		t.Fatal("duplicate generation")
	}
	now = *ready.ExpiresAt
	if _, _, e := s.Download(ctx, tUser.ID, nil, out.ID); !errors.Is(e, ErrCSVExpired) {
		t.Fatal(e)
	}
	if st.opens != 2 {
		t.Fatal("expired download touched storage")
	}
	st.deleteErr = true
	if e := s.Delete(ctx, tUser.ID, nil, out.ID); e == nil {
		t.Fatal("lost cleanup failure")
	}
	retained, e := tClient.CsvExport.Get(ctx, out.ID)
	if e != nil || retained.Status != csvexport.StatusDeleting || retained.ArtifactPath == "" {
		t.Fatal(retained, e)
	}
	if _, _, e := s.Download(ctx, tUser.ID, nil, out.ID); e == nil {
		t.Fatal("deleting download")
	}
	st.deleteErr = false
	if e := s.remove(ctx, retained); e != nil {
		t.Fatal(e)
	}
	if len(st.objects) != 0 {
		t.Fatal("orphan")
	}
	if e := s.Delete(ctx, tUser.ID, nil, out.ID); e != nil {
		t.Fatal("not idempotent", e)
	}
}

func TestCSVFailureRetryAndCrashRecovery(t *testing.T) {
	ctx := context.Background()
	s, st := csvTestService()
	out, e := s.Create(ctx, tUser.ID, nil, csvDraftRequest())
	if e != nil {
		t.Fatal(e)
	}
	defer tClient.CsvExport.DeleteOneID(out.ID).Exec(ctx)
	row, _ := tClient.CsvExport.Get(ctx, out.ID)
	st.putErr = true
	if e := s.run(ctx, row); e == nil {
		t.Fatal("failure hidden")
	}
	failed, e := s.Get(ctx, tUser.ID, nil, out.ID)
	if e != nil || failed.Status != "failed" || strings.Contains(failed.Error, "secret") {
		t.Fatal(failed, e)
	}
	persisted, _ := tClient.CsvExport.Get(ctx, out.ID)
	if persisted.ArtifactPath == "" || persisted.RequestSnapshot.Source == "" {
		t.Fatal("recovery metadata missing")
	}
	if _, _, e := s.Download(ctx, tUser.ID, nil, out.ID); !errors.Is(e, ErrCSVNotReady) {
		t.Fatal(e)
	}
	if e := s.Retry(ctx, tUser.ID, nil, out.ID); e != nil {
		t.Fatal(e)
	}
	st.putErr = false
	row, _ = tClient.CsvExport.Get(ctx, out.ID)
	if e := s.run(ctx, row); e != nil {
		t.Fatal(e)
	}
	if e := s.Retry(ctx, tUser.ID, nil, out.ID); !errors.Is(e, ErrCSVNotReady) {
		t.Fatal("regenerated successful CSV", e)
	}
	if e := s.Delete(ctx, tUser.ID, nil, out.ID); e != nil {
		t.Fatal(e)
	}
	// Simulate an abandoned worker after its enforced deadline.
	out, e = s.Create(ctx, tUser.ID, nil, csvDraftRequest())
	if e != nil {
		t.Fatal(e)
	}
	defer tClient.CsvExport.DeleteOneID(out.ID).Exec(ctx)
	_, e = tClient.CsvExport.UpdateOneID(out.ID).SetStatus(csvexport.StatusRunning).SetArtifactPath("users/" + tUser.ID.String() + "/csv/" + out.ID.String() + ".csv").SetUpdatedAt(time.Now().Add(-11 * time.Minute)).Save(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e := s.Sweep(ctx); e != nil {
		t.Fatal(e)
	}
	ready, e := s.Get(ctx, tUser.ID, nil, out.ID)
	if e != nil || ready.Status != "completed" {
		t.Fatal(ready, e)
	}
	if e := s.Delete(ctx, tUser.ID, nil, out.ID); e != nil {
		t.Fatal(e)
	}
}

func TestCSVScopeAndRevocation(t *testing.T) {
	ctx := context.Background()
	s, st := csvTestService()
	g, e := tRepos.Groups.GroupCreate(ctx, "csv lifecycle", tUser.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer tRepos.Groups.GroupDelete(ctx, g.ID)
	req := csvexports.Request{Source: "entities", Scope: "collection", Boundary: csvexports.Boundary{Kind: "all"}, Columns: []csvexports.Column{{ID: "name"}}}
	out, e := s.Create(ctx, tUser.ID, &g.ID, req)
	if e != nil {
		t.Fatal(e)
	}
	defer tClient.CsvExport.DeleteOneID(out.ID).Exec(ctx)
	row, _ := tClient.CsvExport.Get(ctx, out.ID)
	if e := s.run(ctx, row); e != nil {
		t.Fatal(e)
	}
	if _, _, e := s.Download(ctx, tUser.ID, nil, out.ID); e == nil {
		t.Fatal("collection exposed privately")
	}
	stranger := uuid.New()
	if _, _, e := s.Download(ctx, stranger, &g.ID, out.ID); e == nil {
		t.Fatal("stranger download")
	}
	pending, e := s.Create(ctx, tUser.ID, &g.ID, req)
	if e != nil {
		t.Fatal(e)
	}
	defer tClient.CsvExport.DeleteOneID(pending.ID).Exec(ctx)
	// Membership revocation prevents generation, metadata and blob access.
	_, e = tClient.UserGroup.Delete().Where(usergroup.UserID(tUser.ID), usergroup.GroupID(g.ID)).Exec(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := s.Get(ctx, tUser.ID, &g.ID, out.ID); !errors.Is(e, ErrCSVAccess) {
		t.Fatal(e)
	}
	if _, e := s.List(ctx, tUser.ID, &g.ID); !errors.Is(e, ErrCSVAccess) {
		t.Fatal(e)
	}
	if _, e := s.Create(ctx, tUser.ID, &g.ID, req); !errors.Is(e, ErrCSVAccess) {
		t.Fatal(e)
	}
	if _, _, e := s.Download(ctx, tUser.ID, &g.ID, out.ID); !errors.Is(e, ErrCSVAccess) {
		t.Fatal(e)
	}
	if st.opens != 0 {
		t.Fatal("unauthorized bytes touched")
	}
	pendingRow, _ := tClient.CsvExport.Get(ctx, pending.ID)
	if e := s.run(ctx, pendingRow); e == nil {
		t.Fatal("generated after membership revocation")
	}
	failed, _ := tClient.CsvExport.Get(ctx, pending.ID)
	if failed.Status != csvexport.StatusFailed || st.puts != 1 {
		t.Fatal("revoked source reached storage", failed.Status, st.puts)
	}
	if e := s.remove(ctx, failed); e != nil {
		t.Fatal(e)
	}
	row, _ = tClient.CsvExport.Get(ctx, out.ID)
	if e := s.remove(ctx, row); e != nil {
		t.Fatal(e)
	}
	// The same account has no membership in this group, but private snapshots work.
	private, e := s.Create(ctx, tUser.ID, nil, csvDraftRequest())
	if e != nil {
		t.Fatal(e)
	}
	defer tClient.CsvExport.DeleteOneID(private.ID).Exec(ctx)
	if _, e := s.Get(ctx, stranger, nil, private.ID); e == nil {
		t.Fatal("another user's private artifact")
	}
	shared, e := s.List(ctx, tUser.ID, &tGroup.ID)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range shared {
		if v.ID == private.ID {
			t.Fatal("private in shared history")
		}
	}
	if e := s.Delete(ctx, tUser.ID, nil, private.ID); e != nil {
		t.Fatal(e)
	}
}

func TestCSVRequesterMemberHistoryIsolation(t *testing.T) {
	ctx := context.Background()
	s, _ := csvTestService()
	g, e := tRepos.Groups.GroupCreate(ctx, "csv notifier scope", tUser.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer tRepos.Groups.GroupDelete(ctx, g.ID)
	member, e := tRepos.Users.Create(ctx, repo.UserCreate{Name: "csv member", Email: uuid.NewString() + "@example.com", Password: new("password"), DefaultGroupID: g.ID})
	if e != nil {
		t.Fatal(e)
	}
	defer tRepos.Users.Delete(ctx, member.ID)
	req := csvexports.Request{Source: "notifiers", Scope: "collection", Boundary: csvexports.Boundary{Kind: "all"}, Columns: []csvexports.Column{{ID: "name"}}}
	out, e := s.Create(ctx, tUser.ID, &g.ID, req)
	if e != nil {
		t.Fatal(e)
	}
	defer tClient.CsvExport.DeleteOneID(out.ID).Exec(ctx)
	if _, e := s.Get(ctx, member.ID, &g.ID, out.ID); !errors.Is(e, ErrCSVAccess) {
		t.Fatal(e)
	}
	history, e := s.List(ctx, member.ID, &g.ID)
	if e != nil || len(history) != 0 {
		t.Fatal(history, e)
	}
	if e := s.Delete(ctx, tUser.ID, &g.ID, out.ID); e != nil {
		t.Fatal(e)
	}
}
