package domain

// CandidateDiscoveryCap is the server-owned marketplace discovery bound.
// Search keeps this many visible loads, ordered by created_at DESC, id.
// A client candidate limit may return fewer. It cannot raise this cap.
// Road-distance ranking, matrix batches, and the search time budget are
// later bounds and are not applied by this constant.
const CandidateDiscoveryCap = 1000
