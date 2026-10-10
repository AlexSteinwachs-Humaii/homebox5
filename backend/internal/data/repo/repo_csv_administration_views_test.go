package repo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/csvexports"
)

func TestCsvAdministrationActiveInvitationBoundary(t *testing.T) {
	ctx := context.Background()
	g, err := tRepos.Groups.GroupCreate(ctx, "csv admin views", tUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer tRepos.Groups.GroupDelete(ctx, g.ID)
	ids := []uuid.UUID{}
	for i := 0; i < 55; i++ {
		invitation, err := tRepos.Groups.InvitationCreate(ctx, g.ID, GroupInvitationCreate{Token: []byte(fmt.Sprintf("secret-invite-%d", i)), ExpiresAt: time.Now().Add(time.Hour), Uses: i + 1})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, invitation.ID)
	}
	_, err = tRepos.Groups.InvitationCreate(ctx, g.ID, GroupInvitationCreate{Token: []byte("expired-secret"), ExpiresAt: time.Now().Add(-time.Hour), Uses: 1})
	if err != nil {
		t.Fatal(err)
	}
	// UI filters active rows across the complete list and merges local rows. No
	// plaintext token, page limit or owner flag is part of this request.
	req := csvexports.Request{Source: "invitations", Scope: "collection", Boundary: csvexports.Boundary{Kind: "explicit", RecordIDs: ids}, Columns: []csvexports.Column{{ID: "expiresAt"}, {ID: "remainingUses"}}}
	rows, err := tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 55 {
		t.Fatalf("full active set: %d %v", len(rows), err)
	}
	for i, row := range rows {
		if row["id"] != ids[i].String() || row["remainingUses"] != i+1 {
			t.Fatalf("view order/metadata mismatch: %#v", row)
		}
		if _, ok := row["token"]; ok {
			t.Fatal("token leaked")
		}
	}
	req.Boundary.RecordIDs = nil
	rows, err = tRepos.CsvExports.ResolveSource(ctx, tUser.ID, &g.ID, req)
	if err != nil || len(rows) != 0 {
		t.Fatal("empty visible boundary broadened", rows, err)
	}
	if _, err = tRepos.CsvExports.ResolveSource(ctx, uuid.New(), &g.ID, req); err == nil {
		t.Fatal("unrelated user accepted")
	}
}
