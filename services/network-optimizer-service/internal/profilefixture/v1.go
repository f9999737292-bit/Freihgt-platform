// Package profilefixture holds the v1 system profiles used to bootstrap tests.
// Production scoring reads network_optimizer.score_profiles. This package is not
// a second production configuration source.
package profilefixture

import (
	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/domain"
)

func V1() []domain.ScoreProfile {
	active := func(id, code string, components []domain.ScoreProfileComponent) domain.ScoreProfile {
		return domain.ScoreProfile{
			ID: uuid.MustParse(id), Code: code, Scope: domain.ProfileScopeSystem, Version: 1,
			Status: domain.ProfileStatusActive, AlgorithmVersion: domain.ScoringAlgorithmVersion, Components: components,
		}
	}
	comp := func(code string, weight, ordinal int, required bool) domain.ScoreProfileComponent {
		return domain.ScoreProfileComponent{Code: code, WeightBps: weight, Required: required, Ordinal: ordinal}
	}
	return []domain.ScoreProfile{
		active("00000000-0000-4000-8000-000000000201", domain.ProfileMinDeadhead, []domain.ScoreProfileComponent{
			comp(domain.ComponentDeadheadEfficiency, 10000, 1, true),
		}),
		active("00000000-0000-4000-8000-000000000202", domain.ProfileMaxCapacityUtilization, []domain.ScoreProfileComponent{
			comp(domain.ComponentCapacityUtilization, 10000, 1, true),
		}),
		active("00000000-0000-4000-8000-000000000203", domain.ProfileReturnHome, []domain.ScoreProfileComponent{
			comp(domain.ComponentTargetProximity, 10000, 1, true),
		}),
		active("00000000-0000-4000-8000-000000000204", domain.ProfileMaxRevenue, []domain.ScoreProfileComponent{
			comp(domain.ComponentRevenue, 10000, 1, true),
		}),
		active("00000000-0000-4000-8000-000000000205", domain.ProfileMinRisk, []domain.ScoreProfileComponent{
			comp(domain.ComponentPickupSlack, 5000, 1, true),
			comp(domain.ComponentPredictionConfidence, 3000, 2, false),
			comp(domain.ComponentETAUncertainty, 2000, 3, false),
		}),
		active("00000000-0000-4000-8000-000000000206", domain.ProfileBalanced, []domain.ScoreProfileComponent{
			comp(domain.ComponentDeadheadEfficiency, 3500, 1, true),
			comp(domain.ComponentCapacityUtilization, 2500, 2, false),
			comp(domain.ComponentWaitingEfficiency, 1500, 3, false),
			comp(domain.ComponentTargetProximity, 1000, 4, false),
			comp(domain.ComponentPickupSlack, 1000, 5, false),
			comp(domain.ComponentNetworkValue, 500, 6, false),
		}),
		{
			ID: uuid.MustParse("00000000-0000-4000-8000-000000000207"), Code: domain.ProfileMaxContribution,
			Scope: domain.ProfileScopeSystem, Version: 1, Status: domain.ProfileStatusReserved, AlgorithmVersion: domain.ScoringAlgorithmVersion,
		},
	}
}
