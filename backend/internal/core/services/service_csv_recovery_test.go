package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/csvexport"
)

type csvCompletionFailureKey struct{}

func TestCSVCompletionDatabaseFailureKeepsPointer(t *testing.T) {
	// Scope the failure hook to this job so other shared-database tests are unchanged.
	tClient.CsvExport.Use(func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			if ctx.Value(csvCompletionFailureKey{}) != nil {
				if mutation, ok := m.(*ent.CsvExportMutation); ok {
					status, _ := mutation.Status()
					if status == csvexport.StatusCompleted {
						return nil, errors.New("injected completion failure")
					}
				}
			}
			return next.Mutate(ctx, m)
		})
	})
	ctx := context.Background()
	s, st := csvTestService()
	out, e := s.Create(ctx, tUser.ID, nil, csvDraftRequest())
	if e != nil {
		t.Fatal(e)
	}
	defer tClient.CsvExport.DeleteOneID(out.ID).Exec(ctx)
	row, _ := tClient.CsvExport.Get(ctx, out.ID)
	if e := s.run(context.WithValue(ctx, csvCompletionFailureKey{}, true), row); e == nil {
		t.Fatal("database failure hidden")
	}
	retained, e := tClient.CsvExport.Get(ctx, out.ID)
	if e != nil || retained.ArtifactPath == "" || retained.Status != csvexport.StatusFailed {
		t.Fatal(retained, e)
	}
	if len(st.objects) != 1 {
		t.Fatal("missing test artifact")
	}
	if _, _, e := s.Download(ctx, tUser.ID, nil, out.ID); e == nil {
		t.Fatal("incomplete DB state served")
	}
	// Using the PRE-generation row for deletion must reload the durable pointer.
	if e := s.remove(ctx, row); e != nil {
		t.Fatal(e)
	}
	if len(st.objects) != 0 {
		t.Fatal("stale snapshot orphaned canonical blob")
	}
}

func TestCSVIdentityPurgeWaitsForStorage(t *testing.T) {
	ctx := context.Background()
	s, st := csvTestService()
	out, e := s.Create(ctx, tUser.ID, nil, csvDraftRequest())
	if e != nil {
		t.Fatal(e)
	}
	defer tClient.CsvExport.DeleteOneID(out.ID).Exec(ctx)
	row, _ := tClient.CsvExport.Get(ctx, out.ID)
	if e := s.run(ctx, row); e != nil {
		t.Fatal(e)
	}
	st.deleteErr = true
	if e := s.PurgeIdentity(ctx, &tUser.ID, nil); e == nil {
		t.Fatal("purge ignored failed cleanup")
	}
	persisted, e := tClient.CsvExport.Get(ctx, out.ID)
	if e != nil || persisted.ArtifactPath == "" || persisted.Status != csvexport.StatusDeleting {
		t.Fatal(persisted, e)
	}
	exists, e := tClient.User.Query().Exist(ctx)
	if e != nil || !exists {
		t.Fatal("identity removed first")
	}
	st.deleteErr = false
	if e := s.PurgeIdentity(ctx, &tUser.ID, nil); e != nil {
		t.Fatal(e)
	}
	if len(st.objects) != 0 {
		t.Fatal("purge orphan")
	}
}

func TestCSVNotPortableZIPState(t *testing.T) {
	for _, spec := range exportTables {
		if spec.name == "csv_exports" || spec.name == "users" || spec.name == "groups" {
			t.Fatalf("CSV history or policy entered portable ZIP: %s", spec.name)
		}
	}
}

func TestCSVLateWorkerCannotFailNewClaim(t *testing.T) {
	ctx := context.Background()
	s, st := csvTestService()
	out, e := s.Create(ctx, tUser.ID, nil, csvDraftRequest())
	if e != nil {
		t.Fatal(e)
	}
	defer tClient.CsvExport.DeleteOneID(out.ID).Exec(ctx)
	row, _ := tClient.CsvExport.Get(ctx, out.ID)
	st.putErr = true
	st.beforePut = func() {
		// Model an abandoned claim replaced by a new worker. Old failure must be
		// fenced even though the new attempt has the same running status and key.
		current, e := tClient.CsvExport.Get(ctx, out.ID)
		if e != nil {
			t.Fatal(e)
		}
		e = tClient.CsvExport.UpdateOneID(out.ID).SetUpdatedAt(current.UpdatedAt.Add(time.Minute)).Exec(ctx)
		if e != nil {
			t.Fatal(e)
		}
	}
	if e := s.run(ctx, row); e == nil {
		t.Fatal("storage failure hidden")
	}
	current, _ := tClient.CsvExport.Get(ctx, out.ID)
	if current.Status != csvexport.StatusRunning {
		t.Fatal("late worker mutated newer claim", current.Status)
	}
	// Remove the simulated claim so cleanup can finish the test.
	if e := tClient.CsvExport.UpdateOneID(out.ID).SetStatus(csvexport.StatusFailed).Exec(ctx); e != nil {
		t.Fatal(e)
	}
	if e := s.remove(ctx, current); e != nil {
		t.Fatal(e)
	}
}
