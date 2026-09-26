package repository

import (
	"context"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

type ScoreProfileStore interface {
	ActiveScoreProfile(ctx context.Context, code string) (domain.ScoreProfile, error)
}

func (m *Memory) SeedScoreProfiles(profiles []domain.ScoreProfile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scoreProfiles == nil {
		m.scoreProfiles = map[string]domain.ScoreProfile{}
	}
	for _, profile := range profiles {
		m.scoreProfiles[profile.Code] = cloneProfile(profile)
	}
}

func (m *Memory) ActiveScoreProfile(_ context.Context, code string) (domain.ScoreProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scoreProfiles == nil {
		return domain.ScoreProfile{}, ErrNotFound
	}
	profile, ok := m.scoreProfiles[code]
	if !ok || profile.Status != domain.ProfileStatusActive || profile.Scope != domain.ProfileScopeSystem {
		return domain.ScoreProfile{}, ErrNotFound
	}
	profile = cloneProfile(profile)
	if err := domain.ValidateActiveScoreProfile(profile); err != nil {
		return domain.ScoreProfile{}, err
	}
	return profile, nil
}

func (m *Memory) ReplaceScoreProfile(profile domain.ScoreProfile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scoreProfiles == nil {
		m.scoreProfiles = map[string]domain.ScoreProfile{}
	}
	m.scoreProfiles[profile.Code] = cloneProfile(profile)
}

func cloneProfile(profile domain.ScoreProfile) domain.ScoreProfile {
	profile.Components = append([]domain.ScoreProfileComponent(nil), profile.Components...)
	return profile
}

func (p *Postgres) ActiveScoreProfile(ctx context.Context, code string) (domain.ScoreProfile, error) {
	var profile domain.ScoreProfile
	err := p.pool.QueryRow(ctx, `
		SELECT id, code, scope, version, status, algorithm_version
		FROM network_optimizer.score_profiles
		WHERE code=$1 AND scope='SYSTEM' AND status='ACTIVE' AND tenant_id IS NULL`, code).Scan(
		&profile.ID, &profile.Code, &profile.Scope, &profile.Version, &profile.Status, &profile.AlgorithmVersion)
	if err != nil {
		return domain.ScoreProfile{}, ErrNotFound
	}
	rows, err := p.pool.Query(ctx, `
		SELECT component_code, weight_bps, required, ordinal
		FROM network_optimizer.score_profile_components
		WHERE profile_id=$1
		ORDER BY ordinal, component_code`, profile.ID)
	if err != nil {
		return domain.ScoreProfile{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var component domain.ScoreProfileComponent
		if err := rows.Scan(&component.Code, &component.WeightBps, &component.Required, &component.Ordinal); err != nil {
			return domain.ScoreProfile{}, err
		}
		profile.Components = append(profile.Components, component)
	}
	if err := rows.Err(); err != nil {
		return domain.ScoreProfile{}, err
	}
	if err := domain.ValidateActiveScoreProfile(profile); err != nil {
		return domain.ScoreProfile{}, err
	}
	return profile, nil
}
