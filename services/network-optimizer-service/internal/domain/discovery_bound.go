package domain

// CandidateDiscoveryCap is the server-owned marketplace discovery bound.
// Search keeps this many visible loads, ordered by created_at DESC, id.
// A client candidate limit may return fewer. It cannot raise this cap.
// The routing-stage bounds are separate constants in routing_bound.go.
const CandidateDiscoveryCap = 1000
