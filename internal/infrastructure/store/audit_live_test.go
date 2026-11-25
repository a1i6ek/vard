package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/database/databasetest"
)

// Every entry is one kind of actor and no other, so the three filters split
// the log between them; the count stops where it is told to and says so.
func TestLiveTheLogIsSplitByWhoActed(t *testing.T) {
	db, _ := databasetest.Open(t)
	st := New(db)
	ctx := context.Background()

	admin := uuid.New()
	entries := []model.AuditLog{
		{AdminID: &admin, ActorEmail: "root@example.com", Action: "user.created", TargetType: "user", TargetID: "u1", IP: "10.0.0.1"},
		{AdminID: &admin, ActorEmail: "root@example.com", Action: "application.updated", TargetType: "application", TargetID: "a1", IP: "10.0.0.1"},
		{ActorEmail: "ada@example.com", Action: "user.login", TargetType: "user", TargetID: "u2", IP: "10.0.0.2"},
		{ActorEmail: "ada@example.com", Action: "user.login_failed", TargetType: "user", TargetID: "u2", IP: "10.0.0.2"},
		{Action: "application.logout_abandoned", TargetType: "application", TargetID: "a1"},
	}
	for i := range entries {
		if err := st.WriteAudit(ctx, &entries[i]); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name   string
		filter AuditFilter
		want   int
	}{
		{"by administrators", AuditFilter{ActorTypes: []string{ActorAdmin}}, 2},
		{"by people signing in", AuditFilter{ActorTypes: []string{ActorUser}}, 2},
		{"by the server itself", AuditFilter{ActorTypes: []string{ActorSystem}}, 1},
		{"by people or the server", AuditFilter{ActorTypes: []string{ActorUser, ActorSystem}}, 3},
		{"about applications", AuditFilter{TargetTypes: []string{"application"}}, 2},
		{"from one address", AuditFilter{IP: "10.0.0.2"}, 2},
		{"from part of an address", AuditFilter{IP: "10.0.0"}, 0},
		{"a fragment anywhere", AuditFilter{Search: "LOGIN"}, 2},
		{"a fragment of an address", AuditFilter{Search: "0.0.1"}, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.filter.Limit = 50
			events, _, err := st.AuditQuery(ctx, test.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != test.want {
				t.Errorf("entries = %d, want %d", len(events), test.want)
			}

			count, capped, err := st.AuditCount(ctx, test.filter, 1000)
			if err != nil {
				t.Fatal(err)
			}
			if count != int64(test.want) || capped {
				t.Errorf("count = %d (capped %v), want %d", count, capped, test.want)
			}
		})
	}

	count, capped, err := st.AuditCount(ctx, AuditFilter{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || !capped {
		t.Errorf("a count stopped at 3 = %d (capped %v), want 3, capped", count, capped)
	}
}

// The search matches a fragment anywhere in four columns. On a log of
// millions that must not read every row: with sequential scans priced out,
// the planner reaches each column through the trigram index the migrations
// make, which says the query is written so the index can serve it.
func TestLiveTheLogSearchCanUseItsIndex(t *testing.T) {
	db, _ := databasetest.Open(t)
	st := New(db)
	ctx := context.Background()

	for i := range 50 {
		entry := model.AuditLog{ActorEmail: "someone@example.com", Action: "user.updated",
			TargetType: "user", TargetID: uuid.NewString(), IP: "10.0.0.1", CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute)}
		if err := st.WriteAudit(ctx, &entry); err != nil {
			t.Fatal(err)
		}
	}

	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.Exec("SET LOCAL enable_seqscan = off").Error; err != nil {
		t.Fatal(err)
	}

	// The store's own query, written out by a dry run, then explained.
	dry := New(tx).filtered(ctx, AuditFilter{Search: "someone"}).Select("id").
		Session(&gorm.Session{DryRun: true}).Find(&[]model.AuditLog{}).Statement

	var lines []string
	if err := tx.Raw("EXPLAIN "+dry.SQL.String(), dry.Vars...).Scan(&lines).Error; err != nil {
		t.Fatal(err)
	}
	if text := strings.Join(lines, "\n"); !strings.Contains(text, "idx_audit_logs_search") {
		t.Errorf("the search does not use idx_audit_logs_search:\n%s", text)
	}
}
