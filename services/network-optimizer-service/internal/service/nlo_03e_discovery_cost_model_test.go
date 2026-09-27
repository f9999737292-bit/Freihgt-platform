//go:build nlo03ediscovery

package service

import "testing"

// Deterministic architecture calculation. Not a production generator.
// Inner work follows evaluateGroupageItems: K cargo-vs-equipment checks
// plus C(K,2) cargo-pair checks, once per compatibility context.

func TestNLO03ECostModel(t *testing.T) {
	const pool = 10
	const contexts = 2 // capacity tenant and load tenant, the maximum
	c2, c3, c4, c5 := binom(pool, 2), binom(pool, 3), binom(pool, 4), binom(pool, 5)
	if c2 != 45 || c3 != 120 || c4 != 210 || c5 != 252 {
		t.Fatalf("combinations %d %d %d %d", c2, c3, c4, c5)
	}
	if c2+c3 != 165 || c2+c3+c4 != 375 || c2+c3+c4+c5 != 627 {
		t.Fatalf("totals %d %d %d", c2+c3, c2+c3+c4, c2+c3+c4+c5)
	}
	// Measured size-2 response for 90 member rows. Scaling by members is an estimate.
	const measuredResponseBytes = 104091
	const measuredMembers = 90
	cases := []struct {
		max, sets, members, groupage, inner, estimatedResponse int
	}{
		{3, 165, 450, 330, 1710, 520455},
		{4, 375, 1290, 750, 5910, 1491971},
		{5, 627, 2550, 1254, 13470, 2949245},
	}
	for _, want := range cases {
		got := costThrough(pool, want.max, contexts)
		if got.sets != want.sets || got.members != want.members || got.groupage != want.groupage || got.inner != want.inner {
			t.Fatalf("max %d got sets %d members %d groupage %d inner %d", want.max, got.sets, got.members, got.groupage, got.inner)
		}
		if got.candidates != want.sets || got.persisted != 1+want.sets+want.members {
			t.Fatalf("max %d persisted %d", want.max, got.persisted)
		}
		estimated := measuredResponseBytes * got.members / measuredMembers
		if estimated != want.estimatedResponse {
			t.Fatalf("max %d response estimate %d", want.max, estimated)
		}
		if got.inner <= got.groupage {
			t.Fatalf("max %d inner work must exceed groupage calls", want.max)
		}
	}
}

type setCost struct {
	sets, candidates, members, persisted, groupage, inner int
}

func costThrough(pool, maxSet, contexts int) setCost {
	var out setCost
	for k := 2; k <= maxSet; k++ {
		sets := binom(pool, k)
		perContext := k + binom(k, 2)
		out.sets += sets
		out.members += k * sets
		out.inner += sets * contexts * perContext
	}
	out.candidates = out.sets
	out.groupage = out.sets * contexts
	out.persisted = 1 + out.candidates + out.members
	return out
}

func binom(n, k int) int {
	if k < 0 || k > n {
		return 0
	}
	if k > n-k {
		k = n - k
	}
	result := 1
	for i := 1; i <= k; i++ {
		result = result * (n - k + i) / i
	}
	return result
}
