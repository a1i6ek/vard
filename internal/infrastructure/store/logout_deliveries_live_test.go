package store

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"

	"github.com/a1i6ek/vard/internal/infrastructure/database/databasetest"
	"github.com/a1i6ek/vard/internal/domain/model"
)

// Several instances send back-channel notices at once: each claims its own
// (FOR UPDATE SKIP LOCKED), so no notice is taken twice, and a claimed one
// is left alone until its lease runs out, when it is due again.
func TestLiveEachLogoutNoticeIsClaimedOnce(t *testing.T) {
	db, _ := databasetest.Open(t)
	st := New(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	app := model.Application{Name: "Back office", Profile: model.ProfileWeb, ClientID: "back-office", TokenAuthMethod: model.AuthClientSecretBasic,
		IsEnabled: true, BackchannelLogoutURI: "https://back.example.com/logout", IDTokenSigningAlg: model.AlgRS256}
	if err := db.Omit(clause.Associations).Create(&app).Error; err != nil {
		t.Fatal(err)
	}
	notices := make([]LogoutNotice, 40)
	for i := range notices {
		notices[i] = LogoutNotice{ApplicationID: app.ID, SessionID: uuid.New(), UserID: uuid.New()}
	}
	if err := queueLogoutsIn(db, notices, now); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var claimed []uuid.UUID
	var instances sync.WaitGroup
	for range 4 {
		instances.Go(func() {
			for {
				batch, err := st.ClaimLogouts(ctx, now, time.Minute, 3)
				if err != nil {
					t.Error(err)
					return
				}
				if len(batch) == 0 {
					return
				}
				mu.Lock()
				for _, notice := range batch {
					claimed = append(claimed, notice.ID)
				}
				mu.Unlock()
			}
		})
	}
	instances.Wait()

	slices.SortFunc(claimed, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	if len(claimed) != len(notices) || len(slices.Compact(slices.Clone(claimed))) != len(notices) {
		t.Fatalf("claimed %d notices, %d of them distinct; want each of %d once", len(claimed), len(slices.Compact(claimed)), len(notices))
	}

	if batch, _ := st.ClaimLogouts(ctx, now.Add(30*time.Second), time.Minute, 100); len(batch) != 0 {
		t.Errorf("within the lease, %d notices were claimed again", len(batch))
	}
	batch, err := st.ClaimLogouts(ctx, now.Add(2*time.Minute), time.Minute, 100)
	if err != nil || len(batch) != len(notices) || batch[0].Tries != 2 {
		t.Errorf("past the lease: %d claimed (tries %v), %v; want every one again, on its second try", len(batch), triesOf(batch), err)
	}
}

func triesOf(batch []LogoutNotice) int {
	if len(batch) == 0 {
		return 0
	}
	return batch[0].Tries
}

func TestALogoutIsTriedLessOftenEachTime(t *testing.T) {
	tests := []struct {
		tries int
		want  time.Duration
	}{
		{1, 5 * time.Second},
		{2, 10 * time.Second},
		{5, 80 * time.Second},
		{11, time.Hour},
		{40, time.Hour},
	}
	for _, tt := range tests {
		if got := model.LogoutRetryAfter(tt.tries); got != tt.want {
			t.Errorf("after %d tries: wait %v, want %v", tt.tries, got, tt.want)
		}
	}
}
