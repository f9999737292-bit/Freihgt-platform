package controltower

import (
	"context"
	"log/slog"
	"time"

	"github.com/freight-platform/api-gateway/internal/controltower/legacyaggregate"
	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func (s *Service) applyReadModelStatusSummary(
	ctx context.Context,
	response *SummaryResponse,
	reqCtx RequestContext,
	legacyInput controltowerreadmodel.LegacyStatusInput,
) {
	var (
		rmPayload *controltowerreadmodel.RemoteStatusSummary
		rmErr     *controltowerreadmodel.DependencyError
	)

	if s.readModelCfg.Mode.Enabled() && s.readModel != nil {
		rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
		rmPayload, rmErr = s.readModel.FetchStatusSummary(rmCtx, s.readModelCfg.Mode, reqCtx.TenantID, reqCtx.RequestID)
		cancel()
	}

	mergeOut := controltowerreadmodel.Merge(controltowerreadmodel.MergeInput{
		Mode:                   s.readModelCfg.Mode,
		Legacy:                 legacyInput,
		ReadModel:              rmPayload,
		ReadModelErr:           rmErr,
		RequireConsumerRunning: s.readModelCfg.RequireConsumerRunning,
	})

	mode := string(s.readModelCfg.Mode)
	switch s.readModelCfg.Mode {
	case controltowerreadmodel.ModeShadow:
		if mergeOut.Comparison != "" {
			s.metrics.ObserveComparison(mode, mergeOut.Comparison)
		}
		if rmPayload != nil && rmPayload.IncompleteProjections > 0 {
			s.metrics.ObservePartial(mode)
		}
		applyStatusSummaryMerge(response, mergeOut)
		s.logReadModelShadow(reqCtx, legacyInput, mergeOut, rmErr)
	case controltowerreadmodel.ModePrimary:
		applyStatusSummaryMerge(response, mergeOut)
		if mergeOut.StatusSummaryFreshness != nil && mergeOut.StatusSummaryFreshness.FallbackUsed {
			s.metrics.ObserveFallback(mode, string(mergeOut.FailureReason))
			fallbackLevel := legacyaggregate.FallbackLevelFullAggregate
			if legacyInput.LimitedDataset {
				fallbackLevel = legacyaggregate.FallbackLevelPageLimited
			}
			s.legacyMetrics.ObserveFallback(mode, fallbackLevel, string(mergeOut.FailureReason))
		}
		if rmPayload != nil && rmPayload.IncompleteProjections > 0 && mergeOut.StatusSummaryFreshness != nil && !mergeOut.StatusSummaryFreshness.FallbackUsed {
			s.metrics.ObservePartial(mode)
		}
		s.logReadModelPrimary(reqCtx, mergeOut, rmErr)
	case controltowerreadmodel.ModeDisabled:
		applyStatusSummaryMerge(response, mergeOut)
	default:
		applyStatusSummaryMerge(response, mergeOut)
	}
}

func applyStatusSummaryMerge(response *SummaryResponse, mergeOut controltowerreadmodel.MergeOutput) {
	if mergeOut.StatusSummary != nil {
		response.StatusSummary = &StatusSummaryBlock{
			TotalShipments:        mergeOut.StatusSummary.TotalShipments,
			CountedShipments:      mergeOut.StatusSummary.CountedShipments,
			ByStatus:              mergeOut.StatusSummary.ByStatus,
			IncompleteProjections: mergeOut.StatusSummary.IncompleteProjections,
			Source:                mergeOut.StatusSummary.Source,
			LimitedDataset:        mergeOut.StatusSummary.LimitedDataset,
		}
	}
	if mergeOut.StatusSummaryFreshness != nil {
		response.StatusSummaryFreshness = mapFreshnessBlock(mergeOut.StatusSummaryFreshness)
	}
	if len(mergeOut.Warnings) > 0 {
		response.DataFreshness.Warnings = controltowerreadmodel.AppendUniqueWarnings(
			response.DataFreshness.Warnings,
			mergeOut.Warnings,
		)
		response.DataFreshness.Partial = true
	}
}

func mapFreshnessBlock(in *controltowerreadmodel.StatusSummaryFreshness) *StatusSummaryFreshnessBlock {
	if in == nil {
		return nil
	}
	block := &StatusSummaryFreshnessBlock{
		Loaded:                in.Loaded,
		FallbackUsed:          in.FallbackUsed,
		Partial:               in.Partial,
		Source:                in.Source,
		ConsumerRunning:       in.ConsumerRunning,
		LegacyAggregateLoaded: in.LegacyAggregateLoaded,
	}
	if in.LastRecordReceivedAt != nil {
		formatted := in.LastRecordReceivedAt.UTC().Format(time.RFC3339)
		block.LastRecordReceivedAt = &formatted
	}
	if in.LastProjectionAppliedAt != nil {
		formatted := in.LastProjectionAppliedAt.UTC().Format(time.RFC3339)
		block.LastProjectionAppliedAt = &formatted
	}
	return block
}

func (s *Service) logReadModelShadow(reqCtx RequestContext, legacy controltowerreadmodel.LegacyStatusInput, out controltowerreadmodel.MergeOutput, rmErr *controltowerreadmodel.DependencyError) {
	if s.readModelLog == nil {
		return
	}
	attrs := []any{
		slog.String("mode", string(s.readModelCfg.Mode)),
		slog.String("comparison", string(out.Comparison)),
		slog.String("request_id", reqCtx.RequestID),
		slog.Bool("legacy_aggregate_loaded", legacy.FullAggregateAvailable),
		slog.Bool("legacy_limited_dataset", legacy.LimitedDataset),
	}
	if rmErr != nil {
		attrs = append(attrs, slog.String("reason", string(rmErr.Reason)))
	}
	s.readModelLog.Info("control_tower_read_model_shadow_comparison", attrs...)
}

func (s *Service) logReadModelPrimary(reqCtx RequestContext, out controltowerreadmodel.MergeOutput, rmErr *controltowerreadmodel.DependencyError) {
	if s.readModelLog == nil {
		return
	}
	attrs := []any{
		slog.String("mode", string(s.readModelCfg.Mode)),
		slog.String("request_id", reqCtx.RequestID),
	}
	if out.StatusSummaryFreshness != nil {
		attrs = append(attrs,
			slog.Bool("fallback_used", out.StatusSummaryFreshness.FallbackUsed),
			slog.Bool("partial", out.StatusSummaryFreshness.Partial),
		)
		if out.StatusSummaryFreshness.LegacyAggregateLoaded != nil {
			attrs = append(attrs, slog.Bool("legacy_aggregate_loaded", *out.StatusSummaryFreshness.LegacyAggregateLoaded))
		}
	}
	if rmErr != nil {
		attrs = append(attrs, slog.String("reason", string(rmErr.Reason)))
	}
	s.readModelLog.Info("control_tower_read_model_primary_merge", attrs...)
}
