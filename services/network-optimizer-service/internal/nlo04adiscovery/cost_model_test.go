package nlo04adiscovery

import "testing"

// Discovery-only combinatorial model. Not a planner and not a production API.
// A future route of N stops accepts one additional load by choosing a pickup
// gap and a delivery gap at or after that pickup. The count is (N+1)*(N+2)/2.

func insertionPairs(futureStops int) int {
	n := 0
	for pickup := 0; pickup <= futureStops; pickup++ {
		for delivery := pickup; delivery <= futureStops; delivery++ {
			n++
		}
	}
	return n
}

func place(stops []int, pickupGap, deliveryGap, pickupLoc, deliveryLoc int) []int {
	out := make([]int, 0, len(stops)+2)
	for gap := 0; gap <= len(stops); gap++ {
		if gap == pickupGap {
			out = append(out, pickupLoc)
		}
		if gap == deliveryGap {
			out = append(out, deliveryLoc)
		}
		if gap < len(stops) {
			out = append(out, stops[gap])
		}
	}
	return out
}

type searchCost struct {
	candidates  int
	legsNoCache int
	uniqueLegs  int
	finalStops  int
}

func incrementalInsertion(startStops, loads int) searchCost {
	current := make([]int, startStops)
	for i := range current {
		current[i] = i + 1
	}
	nextLoc := 1000
	seen := map[[2]int]struct{}{}
	var cost searchCost
	for load := 0; load < loads; load++ {
		pickupLoc := nextLoc
		deliveryLoc := nextLoc + 1
		nextLoc += 2
		var winner []int
		for pickup := 0; pickup <= len(current); pickup++ {
			for delivery := pickup; delivery <= len(current); delivery++ {
				seq := place(current, pickup, delivery, pickupLoc, deliveryLoc)
				cost.candidates++
				for i := 0; i < len(seq)-1; i++ {
					cost.legsNoCache++
					seen[[2]int{seq[i], seq[i+1]}] = struct{}{}
				}
				if winner == nil {
					winner = seq
				}
			}
		}
		current = winner
	}
	cost.uniqueLegs = len(seen)
	cost.finalStops = len(current)
	return cost
}

func TestNLO04AInsertionCostModel(t *testing.T) {
	if insertionPairs(0) != 1 || insertionPairs(2) != 6 || insertionPairs(4) != 15 || insertionPairs(8) != 45 {
		t.Fatalf("pairs 0=%d 2=%d 4=%d 8=%d", insertionPairs(0), insertionPairs(2), insertionPairs(4), insertionPairs(8))
	}
	// 0.3D shape: current position and destination are the two anchored future stops.
	one := incrementalInsertion(2, 1)
	two := incrementalInsertion(2, 2)
	three := incrementalInsertion(2, 3)
	if one.candidates != 6 || one.legsNoCache != 18 || one.finalStops != 4 {
		t.Fatalf("anchor+1 %+v", one)
	}
	if two.candidates != 21 || two.legsNoCache != 93 || two.finalStops != 6 {
		t.Fatalf("anchor+2 %+v", two)
	}
	if three.candidates != 49 || three.legsNoCache != 289 || three.finalStops != 8 {
		t.Fatalf("anchor+3 %+v", three)
	}
	// Richer remaining route. Two additional loads stay under 300 uncached route legs.
	// Three additional loads do not.
	richTwo := incrementalInsertion(4, 2)
	richThree := incrementalInsertion(4, 3)
	if richTwo.legsNoCache > 300 || richThree.legsNoCache <= 300 {
		t.Fatalf("rich +2 %+v +3 %+v", richTwo, richThree)
	}
	if richTwo.uniqueLegs >= richTwo.legsNoCache {
		t.Fatalf("leg reuse did not reduce calls: %+v", richTwo)
	}
	t.Logf("ANCHOR stops=2 +1 candidates=%d legs=%d unique=%d", one.candidates, one.legsNoCache, one.uniqueLegs)
	t.Logf("ANCHOR stops=2 +2 candidates=%d legs=%d unique=%d", two.candidates, two.legsNoCache, two.uniqueLegs)
	t.Logf("ANCHOR stops=2 +3 candidates=%d legs=%d unique=%d", three.candidates, three.legsNoCache, three.uniqueLegs)
	t.Logf("RICH stops=4 +2 candidates=%d legs=%d unique=%d stops=%d", richTwo.candidates, richTwo.legsNoCache, richTwo.uniqueLegs, richTwo.finalStops)
	t.Logf("RICH stops=4 +3 candidates=%d legs=%d unique=%d stops=%d", richThree.candidates, richThree.legsNoCache, richThree.uniqueLegs, richThree.finalStops)
}
