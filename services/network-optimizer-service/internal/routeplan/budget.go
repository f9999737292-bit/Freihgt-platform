package routeplan

import (
	"fmt"
	"time"
)

type SearchError struct {
	Code   string
	Budget string
	Detail string
}

func (e *SearchError) Error() string {
	if e.Budget != "" {
		return fmt.Sprintf("%s budget=%s", e.Code, e.Budget)
	}
	return e.Code
}

type Budget struct {
	SequenceCandidates   int
	RouteLegEvaluations  int
	RoutingProviderCalls int
	GroupageEvaluations  int
	Started              time.Time
	BlockedReason        string
	now                  func() time.Time
}

func NewBudget(now func() time.Time) *Budget {
	if now == nil {
		now = time.Now
	}
	return &Budget{Started: now(), now: now}
}

func (b *Budget) elapsed() time.Duration {
	if b.now == nil {
		return time.Since(b.Started)
	}
	return b.now().Sub(b.Started)
}

func (b *Budget) before(kind string) error {
	if b.BlockedReason != "" {
		return &SearchError{Code: ResultBudget, Budget: b.BlockedReason, Detail: ReasonBudgetExceeded}
	}
	if b.elapsed() > TimeBudget {
		b.BlockedReason = BudgetTime
		return &SearchError{Code: ResultBudget, Budget: BudgetTime, Detail: ReasonBudgetExceeded}
	}
	var current, limit int
	switch kind {
	case BudgetSequences:
		current, limit = b.SequenceCandidates, MaxSequenceCandidates
	case BudgetRouting:
		current, limit = b.RoutingProviderCalls, MaxRoutingProviderCalls
	case BudgetGroupage:
		current, limit = b.GroupageEvaluations, MaxGroupageEvaluations
	case BudgetTime:
		return nil
	default:
		current, limit = b.RouteLegEvaluations, MaxRouteLegEvaluations
		kind = "legs"
	}
	if kind == "legs" {
		if b.RouteLegEvaluations >= MaxRouteLegEvaluations {
			b.BlockedReason = "routing"
			return &SearchError{Code: ResultBudget, Budget: "routing", Detail: ReasonBudgetExceeded}
		}
		return nil
	}
	if current >= limit {
		b.BlockedReason = kind
		return &SearchError{Code: ResultBudget, Budget: kind, Detail: ReasonBudgetExceeded}
	}
	return nil
}

func (b *Budget) openSequence() error {
	if err := b.before(BudgetSequences); err != nil {
		return err
	}
	b.SequenceCandidates++
	return nil
}

func (b *Budget) openLeg() error {
	if err := b.before("legs"); err != nil {
		return err
	}
	b.RouteLegEvaluations++
	return nil
}

func (b *Budget) openProvider() error {
	if err := b.before(BudgetRouting); err != nil {
		return err
	}
	b.RoutingProviderCalls++
	if b.RoutingProviderCalls > b.RouteLegEvaluations {
		b.BlockedReason = BudgetRouting
		return &SearchError{Code: ResultBudget, Budget: BudgetRouting, Detail: ReasonBudgetExceeded}
	}
	return nil
}

func (b *Budget) openGroupage() error {
	if err := b.before(BudgetGroupage); err != nil {
		return err
	}
	b.GroupageEvaluations++
	return nil
}
