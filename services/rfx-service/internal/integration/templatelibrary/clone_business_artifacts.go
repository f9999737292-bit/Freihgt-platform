//go:build integration

package templatelibrary

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// Discovered clone business-artifact storage mapping (from migrations):
//   - Carrier responses: rfx.rfx_responses (rfx_event_id)
//   - Participants/invitations: rfx.rfx_participants (rfx_event_id)
//   - Offer lines: rfx.rfx_response_offer_lines (via rfx_response_id; offers are part of response)
//   - Carrier answers: rfx.rfx_answers (via rfx_response_id)
//   - Evaluation awards: rfx.rfx_awards (via rfx_response_id)
//   - Qualification results: rfx.rfx_qualification_results (via rfx_response_id)
//   - Answer scores: rfx.rfx_answer_scores (via rfx_response_id)
//   - Scoring models: rfx.rfx_score_models (rfx_version_id)
//   - Scoring criteria: rfx.rfx_score_criteria (via score_model_id)
//   - Scoring bindings: rfx.rfx_score_bindings (via score_model_id)
//
// Bids (rfx.bids) attach to freight_requests, not rfx_events — out of clone artifact scope.

type cloneBusinessArtifactCounts struct {
	responses            int
	participants         int
	offerLines           int
	answers              int
	awards               int
	qualificationResults int
	answerScores         int
	scoreModels          int
	scoreCriteria        int
	scoreBindings        int
}

func countCloneBusinessArtifacts(ctx context.Context, env *testEnv, tenantID, eventID, versionID uuid.UUID) (cloneBusinessArtifactCounts, error) {
	var out cloneBusinessArtifactCounts
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_responses WHERE tenant_id=$1 AND rfx_event_id=$2`, tenantID, eventID).Scan(&out.responses); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_participants WHERE tenant_id=$1 AND rfx_event_id=$2`, tenantID, eventID).Scan(&out.participants); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_response_offer_lines ol
		JOIN rfx.rfx_responses r ON r.id = ol.rfx_response_id AND r.tenant_id = ol.tenant_id
		WHERE r.tenant_id=$1 AND r.rfx_event_id=$2`, tenantID, eventID).Scan(&out.offerLines); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_answers a
		JOIN rfx.rfx_responses r ON r.id = a.rfx_response_id AND r.tenant_id = a.tenant_id
		WHERE r.tenant_id=$1 AND r.rfx_event_id=$2`, tenantID, eventID).Scan(&out.answers); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_awards aw
		JOIN rfx.rfx_responses r ON r.id = aw.rfx_response_id AND r.tenant_id = aw.tenant_id
		WHERE r.tenant_id=$1 AND r.rfx_event_id=$2`, tenantID, eventID).Scan(&out.awards); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_qualification_results qr
		JOIN rfx.rfx_responses r ON r.id = qr.rfx_response_id AND r.tenant_id = qr.tenant_id
		WHERE r.tenant_id=$1 AND r.rfx_event_id=$2`, tenantID, eventID).Scan(&out.qualificationResults); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_answer_scores s
		JOIN rfx.rfx_responses r ON r.id = s.rfx_response_id AND r.tenant_id = s.tenant_id
		WHERE r.tenant_id=$1 AND r.rfx_event_id=$2`, tenantID, eventID).Scan(&out.answerScores); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `SELECT COUNT(*) FROM rfx.rfx_score_models WHERE tenant_id=$1 AND rfx_version_id=$2`, tenantID, versionID).Scan(&out.scoreModels); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_score_criteria sc
		JOIN rfx.rfx_score_models sm ON sm.id = sc.score_model_id AND sm.tenant_id = sc.tenant_id
		WHERE sm.tenant_id=$1 AND sm.rfx_version_id=$2`, tenantID, versionID).Scan(&out.scoreCriteria); err != nil {
		return out, err
	}
	if err := env.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rfx.rfx_score_bindings sb
		JOIN rfx.rfx_score_models sm ON sm.id = sb.score_model_id AND sm.tenant_id = sb.tenant_id
		WHERE sm.tenant_id=$1 AND sm.rfx_version_id=$2`, tenantID, versionID).Scan(&out.scoreBindings); err != nil {
		return out, err
	}
	return out, nil
}

func assertNoCloneBusinessArtifacts(t *testing.T, env *testEnv, fix buyerFixture, eventID, versionID uuid.UUID) {
	t.Helper()
	counts, err := countCloneBusinessArtifacts(context.Background(), env, fix.TenantID, eventID, versionID)
	if err != nil {
		t.Fatalf("count clone business artifacts: %v", err)
	}
	if counts.responses != 0 || counts.participants != 0 || counts.offerLines != 0 || counts.answers != 0 ||
		counts.awards != 0 || counts.qualificationResults != 0 || counts.answerScores != 0 ||
		counts.scoreModels != 0 || counts.scoreCriteria != 0 || counts.scoreBindings != 0 {
		t.Fatalf("unexpected clone business artifacts: responses=%d participants=%d offerLines=%d answers=%d awards=%d qualificationResults=%d answerScores=%d scoreModels=%d scoreCriteria=%d scoreBindings=%d",
			counts.responses, counts.participants, counts.offerLines, counts.answers, counts.awards,
			counts.qualificationResults, counts.answerScores, counts.scoreModels, counts.scoreCriteria, counts.scoreBindings)
	}
}
