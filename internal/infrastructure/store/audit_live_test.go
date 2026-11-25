package store

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/cache/cachetest"
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

// The dashboard counts the days asked for and, apart from them, as many days
// before: a sign-in or a new user lands in one period or the other.
func TestLiveTheOverviewCountsTwoPeriods(t *testing.T) {
	db, _ := databasetest.Open(t)
	st := New(db)
	ctx := context.Background()
	now := time.Now()

	at := func(daysAgo int) time.Time { return now.Add(-time.Duration(daysAgo)*24*time.Hour + time.Hour) }
	for _, entry := range []model.AuditLog{
		{Action: "user.login", TargetType: "user", CreatedAt: at(1)},
		{Action: "user.login_failed", TargetType: "user", CreatedAt: at(2)},
		{Action: "user.login", TargetType: "user", CreatedAt: at(9)},
		{Action: "admin.login_blocked", CreatedAt: at(10)},
		{Action: "user.login", TargetType: "user", CreatedAt: at(20)},
	} {
		if err := st.WriteAudit(ctx, &entry); err != nil {
			t.Fatal(err)
		}
	}
	for i, daysAgo := range []int{1, 3, 8} {
		user := model.User{Email: "person" + strconv.Itoa(i) + "@example.com", IsActive: true}
		user.CreatedAt = at(daysAgo)
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}

	numbers, err := st.OverviewNumbers(ctx, now, 7, 5)
	if err != nil {
		t.Fatal(err)
	}

	if got := numbers.SignIns; got.Succeeded != 1 || got.Failed != 1 || got.Blocked != 0 {
		t.Errorf("this week = %+v, want 1 succeeded and 1 failed", got)
	}
	if got := numbers.Previous.SignIns; got.Succeeded != 1 || got.Failed != 0 || got.Blocked != 1 {
		t.Errorf("the week before = %+v, want 1 succeeded and 1 blocked", got)
	}
	if numbers.Counts.NewUsers != 2 || numbers.Previous.NewUsers != 1 {
		t.Errorf("new users = %d, then %d before; want 2, then 1", numbers.Counts.NewUsers, numbers.Previous.NewUsers)
	}
	if first, err := time.Parse("2006-01-02", numbers.Daily[0].Day); err != nil ||
		numbers.SignIns.Since.Format("2006-01-02") != first.Format("2006-01-02") {
		t.Errorf("sign-ins counted since %v, want the chart's first day %s", numbers.SignIns.Since, numbers.Daily[0].Day)
	}
	if !numbers.Previous.SignIns.Since.Equal(numbers.Previous.Since) {
		t.Errorf("the period before starts %v, its sign-ins %v", numbers.Previous.Since, numbers.Previous.SignIns.Since)
	}
	if len(numbers.Daily) != 7 {
		t.Errorf("days = %d, want 7", len(numbers.Daily))
	}
}

// Every administrator who opens the panel asks for the same numbers, so they
// are read once and kept briefly: a sign-in a moment later is not counted
// until they age out. With no Redis they are read every time.
func TestLiveTheOverviewIsCachedBriefly(t *testing.T) {
	redis := cachetest.Open(t)
	if redis == nil {
		t.Skip("set " + cachetest.Env + " to run against Redis")
	}

	db, _ := databasetest.Open(t)
	ctx := context.Background()

	for _, test := range []struct {
		name   string
		store  *Store
		cached bool
	}{
		{"with Redis", New(db).WithCache(redis), true},
		{"without Redis", New(db), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			first, err := test.store.OverviewNumbers(ctx, time.Now(), 14, 5)
			if err != nil {
				t.Fatal(err)
			}

			if err := test.store.WriteAudit(ctx, &model.AuditLog{Action: "user.login", TargetType: "user"}); err != nil {
				t.Fatal(err)
			}

			second, err := test.store.OverviewNumbers(ctx, time.Now(), 14, 5)
			if err != nil {
				t.Fatal(err)
			}

			grew := second.SignIns.Succeeded == first.SignIns.Succeeded+1
			if grew == test.cached {
				t.Errorf("sign-ins %d, then %d: cached = %v, want %v",
					first.SignIns.Succeeded, second.SignIns.Succeeded, !grew, test.cached)
			}
		})
	}
}
