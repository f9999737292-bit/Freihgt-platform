package chain

import (
	"testing"
	"time"
)

func TestNLO10A_ChainBoundsStayAtDiscovery(t *testing.T) {
	if TotalLoads != 2 || Horizon != 48*time.Hour {
		t.Fatalf("loads %d horizon %s", TotalLoads, Horizon)
	}
	if FirstLoadCandidatesMax != 8 || SecondLoadCandidatesPerFirstMax != 2 || PairCandidatesMax != 4 {
		t.Fatalf("candidates %d %d pairs %d", FirstLoadCandidatesMax, SecondLoadCandidatesPerFirstMax, PairCandidatesMax)
	}
	if RoutePlanEvaluationsMax != 2 || ProviderCallsMax != 24 || WallClockBudget != 20*time.Second {
		t.Fatalf("evaluations %d calls %d budget %s", RoutePlanEvaluationsMax, ProviderCallsMax, WallClockBudget)
	}
}
