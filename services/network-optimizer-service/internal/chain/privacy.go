package chain

import "github.com/google/uuid"

// ProjectedLoad is publication-visible identity already known to this carrier.
// It has no shipper, customer, contract, rate, or foreign shipment id.
type ProjectedLoad struct {
	LoadID           uuid.UUID `json:"load_id"`
	Version          int       `json:"version"`
	PublicationLabel string    `json:"publication_label,omitempty"`
}

// ProjectedCandidate is the pre-accept chain surface.
// RoutePlanID stays nil until a later persistence stage stores an evaluation.
type ProjectedCandidate struct {
	CandidateID       uuid.UUID       `json:"chain_candidate_id"`
	Loads             []ProjectedLoad `json:"loads"`
	SelectedOrder     string          `json:"selected_order,omitempty"`
	OrdersEvaluated   []string        `json:"orders_evaluated,omitempty"`
	Feasible          bool            `json:"feasible"`
	EmptyRoadMeters   *int            `json:"empty_road_meters,omitempty"`
	DurationSeconds   *int            `json:"duration_seconds,omitempty"`
	ReasonCodes       []string        `json:"reason_codes,omitempty"`
	OmittedComponents []string        `json:"omitted_components,omitempty"`
	RoutePlanID       *uuid.UUID      `json:"route_plan_id"`
}

func projectLoad(load VisibleLoad) ProjectedLoad {
	return ProjectedLoad{LoadID: load.ID, Version: load.Version, PublicationLabel: load.PublicationLabel}
}
