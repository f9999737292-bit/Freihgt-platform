package allocationpolicy

import (
	"fmt"

	"github.com/shopspring/decimal"
)

var (
	scoreZero = decimal.RequireFromString("0")
	scoreHundred = decimal.RequireFromString("100")
)

// ScorePart is one weighted contribution before final rounding.
type ScorePart struct {
	Attr   string          `json:"attr"`
	Weight decimal.Decimal `json:"weight"`
	Value  decimal.Decimal `json:"value"`
	Part   decimal.Decimal `json:"part"`
	NodeID string          `json:"node_id,omitempty"`
}

// RoundScoreTotal rounds final score_total to 2 dp half-away-from-zero.
func RoundScoreTotal(raw decimal.Decimal) decimal.Decimal {
	// shopspring Round uses round-half-up; for |x|.xx5 this is away-from-zero on ties.
	return raw.Round(2)
}

func validateComponentScore(v decimal.Decimal) error {
	if v.LessThan(scoreZero) || v.GreaterThan(scoreHundred) {
		return validationErr("SCORE_OUT_OF_RANGE",
			fmt.Sprintf("component score %s outside [0,100]", v.String()))
	}
	return nil
}
