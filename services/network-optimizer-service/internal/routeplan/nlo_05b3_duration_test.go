package routeplan

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNLO05B3MissingServiceDurationIsNotExecutable(t *testing.T) {
	dest := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee1")
	load := loadAt("00000000-0000-0000-0000-0000000000d1", "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeee2", dest.String(), 5)
	out, _, err := Planner{}.Plan(context.Background(), Input{
		Mode: ModeCurrentTrip, Start: Stop{Role: RoleStart, Point: anchor(10, 10)},
		End:   &Stop{Role: RoleEnd, Point: canonical(dest, 11, 11)},
		Loads: []Load{load}, Initial: knownCapacity(50), Clock: time.Unix(0, 0),
		Route: &scriptRoute{}, Groupage: compatibleGroupage,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ExecutionSupported || out.ServiceDurationKnown || out.ServiceDurationSeconds != nil {
		t.Fatalf("executable %+v", out)
	}
	if out.ResultStatus != ResultIndeterminate || !contains(out.ReasonCodes, ReasonServiceDurationUnknown) {
		t.Fatalf("%s %v", out.ResultStatus, out.ReasonCodes)
	}
}
