package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/a1i6ek/vard/internal/model"
)

// TokenExchangeRules lists an API's exchange rules, oldest first.
func (s *Store) TokenExchangeRules(ctx context.Context, apiID uuid.UUID) ([]model.TokenExchangeRule, error) {
	var rules []model.TokenExchangeRule
	err := s.db.WithContext(ctx).Where("api_id = ?", apiID).
		Order("created_at, application_id, subject_application_id").Find(&rules).Error
	return rules, err
}

// SetTokenExchangeRules makes `rules` the API's, in place of whatever were.
func (s *Store) SetTokenExchangeRules(ctx context.Context, apiID uuid.UUID, rules []model.TokenExchangeRule, at time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("api_id = ?", apiID).Delete(&model.TokenExchangeRule{}).Error; err != nil {
			return err
		}
		if len(rules) == 0 {
			return nil
		}
		for i := range rules {
			rules[i].APIID, rules[i].CreatedAt = apiID, at
		}
		return tx.Omit("API", "Application", "SubjectApplication").Create(&rules).Error
	})
}

// TokenExchangeRule is the rule letting the application exchange the
// subject application's tokens for the API's, with the API: ErrNotFound when
// there is none. Read from the database every time: a removal ends what may
// be exchanged at once.
func (s *Store) TokenExchangeRule(ctx context.Context, identifier string, applicationID, subjectID uuid.UUID) (*model.TokenExchangeRule, *model.API, error) {
	var api model.API
	err := s.db.WithContext(ctx).Preload("Scopes").Where("identifier = ?", identifier).First(&api).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	var rule model.TokenExchangeRule
	err = s.db.WithContext(ctx).
		Where("api_id = ? AND application_id = ? AND subject_application_id = ?", api.ID, applicationID, subjectID).
		First(&rule).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &api, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return &rule, &api, nil
}
