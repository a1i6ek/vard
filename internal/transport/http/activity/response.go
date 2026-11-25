package activity

import (
	"time"

	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
)

// targetResponse is what an entry happened to. Name is left out when the
// record is gone or the administrator may not see what it is called.
type targetResponse struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// eventResponse is one entry of the activity list.
type eventResponse struct {
	ID        string          `json:"id"`
	Action    string          `json:"action"`
	Actor     string          `json:"actor"`
	IP        string          `json:"ip"`
	Target    *targetResponse `json:"target"`
	Detail    string          `json:"detail,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// logResponse is the same entry on the logs page, which has room for more:
// everything the panel's detail view shows, so it derives none of it.
type logResponse struct {
	eventResponse
	UserAgent string `json:"user_agent"`
	// ActorType is store.ActorAdmin, ActorUser or ActorSystem.
	ActorType string `json:"actor_type"`
	RequestID string `json:"request_id"`
	// Metadata is what else the entry recorded, left out unless the
	// administrator may see the record it is about (mayReadMetadata).
	Metadata map[string]any `json:"metadata,omitempty"`
}

// logsResponse is one page of the log. Total counts what the filters match
// on the first page only, up to countLimit; Capped says it stopped there.
type logsResponse struct {
	Logs   []logResponse `json:"logs"`
	Next   string        `json:"next"`
	Total  *int64        `json:"total,omitempty"`
	Capped bool          `json:"total_capped,omitempty"`
}

func actorType(event model.AuditLog) string {
	switch {
	case event.AdminID != nil:
		return store.ActorAdmin
	case event.TargetType == "user":
		return store.ActorUser
	default:
		return store.ActorSystem
	}
}

type overviewResponse struct {
	// Days is the range everything below was counted over.
	Days    int           `json:"days"`
	Counts  store.Counts  `json:"counts"`
	SignIns store.SignIns `json:"sign_ins"`
	// Previous is the same number of days just before, to compare with.
	Previous  store.Previous     `json:"previous"`
	Daily     []store.DayCount   `json:"daily"`
	TopActors []store.ActorCount `json:"top_actors"`
	Activity  []eventResponse    `json:"activity"`
}

func newEventResponse(event model.AuditLog) eventResponse {
	return eventResponse{
		ID:        event.ID.String(),
		Action:    event.Action,
		Actor:     event.ActorEmail,
		IP:        event.IP,
		CreatedAt: event.CreatedAt,
	}
}

func newLogResponses(events []model.AuditLog, described []eventResponse, metadata func(model.AuditLog) bool) []logResponse {
	out := make([]logResponse, 0, len(events))
	for i, event := range events {
		entry := logResponse{
			eventResponse: described[i],
			UserAgent:     event.UserAgent,
			ActorType:     actorType(event),
			RequestID:     event.RequestID,
		}
		if len(event.Metadata) > 0 && metadata(event) {
			entry.Metadata = event.Metadata
		}
		out = append(out, entry)
	}

	return out
}
