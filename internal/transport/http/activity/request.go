package activity

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/a1i6ek/vard/internal/domain/model"
	"github.com/a1i6ek/vard/internal/infrastructure/store"
	"github.com/a1i6ek/vard/internal/transport/http/session"
)

const dayLayout = "2006-01-02"

func parseFilter(c *gin.Context, limit int) (store.AuditFilter, error) {
	filter := store.AuditFilter{
		Search: strings.TrimSpace(c.Query("q")),
		Actor:  strings.TrimSpace(c.Query("actor")),
		Limit:  limit,
	}

	if admin := session.Admin(c); admin == nil || !admin.HasPermission(model.PermUsersRead) {
		filter.HideUserActors = true
	}

	actions := c.QueryArray("action")
	if len(actions) > maxActions {
		return filter, filterInvalid.With()
	}
	for _, action := range actions {
		if !actionPattern.MatchString(action) {
			return filter, filterInvalid.With()
		}
	}
	filter.Actions = actions

	kinds := c.QueryArray("actor_type")
	if len(kinds) > len(actorTypes) {
		return filter, filterInvalid.With()
	}
	for _, kind := range kinds {
		if !actorTypes[kind] {
			return filter, filterInvalid.With()
		}
		filter.ActorTypes = append(filter.ActorTypes, kind)
	}

	targetTypes := c.QueryArray("target_type")
	if len(targetTypes) > maxTargetTypes {
		return filter, filterInvalid.With()
	}
	for _, kind := range targetTypes {
		if !targetTypePattern.MatchString(kind) {
			return filter, filterInvalid.With()
		}
	}
	filter.TargetTypes = targetTypes

	// An address is compared whole, so anything longer than one cannot match.
	if filter.IP = strings.TrimSpace(c.Query("ip")); len(filter.IP) > maxIPLength {
		return filter, filterInvalid.With()
	}

	var err error
	if filter.From, err = moment(c.Query("from"), false); err != nil {
		return filter, filterInvalid.With()
	}
	if filter.To, err = moment(c.Query("to"), true); err != nil {
		return filter, filterInvalid.With()
	}

	if raw := c.Query("before"); raw != "" {
		cursor, err := parseCursor(raw)
		if err != nil {
			return filter, filterInvalid.With()
		}
		filter.Before = &cursor
	}

	return filter, nil
}

func moment(raw string, end bool) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}

	if day, err := time.ParseInLocation(dayLayout, raw, time.Local); err == nil {
		if end {
			return day.AddDate(0, 0, 1), nil
		}
		return day, nil
	}

	return time.Parse(time.RFC3339, raw)
}

func formatCursor(cursor *store.AuditCursor) string {
	if cursor == nil {
		return ""
	}

	return strconv.FormatInt(cursor.At.UnixNano(), 10) + "_" + cursor.ID.String()
}

func parseCursor(raw string) (store.AuditCursor, error) {
	at, id, ok := strings.Cut(raw, "_")
	if !ok {
		return store.AuditCursor{}, strconv.ErrSyntax
	}

	nanos, err := strconv.ParseInt(at, 10, 64)
	if err != nil {
		return store.AuditCursor{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return store.AuditCursor{}, err
	}

	return store.AuditCursor{At: time.Unix(0, nanos), ID: parsed}, nil
}
