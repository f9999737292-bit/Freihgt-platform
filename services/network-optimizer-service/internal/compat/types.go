package compat

const (
	StatusCompatible    = "COMPATIBLE"
	StatusIncompatible  = "INCOMPATIBLE"
	StatusIndeterminate = "INDETERMINATE"

	ProvenanceAssetConfirmed   = "ASSET_CONFIRMED"
	ProvenanceCargoConfirmed   = "CARGO_CONFIRMED"
	ProvenanceReferenceDefault = "REFERENCE_DEFAULT"
	ProvenanceTenantOverride   = "TENANT_OVERRIDE"
	ProvenanceUnknown          = "UNKNOWN"

	LayerRegulatory = "REGULATORY"
	LayerPlatform   = "PLATFORM"
	LayerTenant     = "TENANT"

	DecisionAllow             = "ALLOW"
	DecisionDeny              = "DENY"
	DecisionRequireSeparation = "REQUIRE_SEPARATION"
	DecisionRequireCondition  = "REQUIRE_CONDITION"
	KindCargoCargo            = "CARGO_CARGO"
	KindCargoEquipment        = "CARGO_EQUIPMENT"
)

type Cargo struct {
	ID                  string            `json:"id"`
	CargoTypeCode       *string           `json:"cargo_type_code,omitempty"`
	ParentCodes         []string          `json:"parent_codes,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
	WeightKg            *float64          `json:"weight_kg,omitempty"`
	VolumeM3            *float64          `json:"volume_m3,omitempty"`
	PalletCount         *int              `json:"pallet_count,omitempty"`
	PalletTypeCode      *string           `json:"pallet_type_code,omitempty"`
	LinearMeters        *float64          `json:"linear_meters,omitempty"`
	MaxLoadedHeightMM   *int              `json:"max_loaded_height_mm,omitempty"`
	Stackable           *bool             `json:"stackable,omitempty"`
	Fragile             *bool             `json:"fragile,omitempty"`
	PackagingTypeCode   *string           `json:"packaging_type_code,omitempty"`
	FoodGradeRequired   *bool             `json:"food_grade_required,omitempty"`
	TemperatureRequired *bool             `json:"temperature_required,omitempty"`
	TemperatureMinC     *float64          `json:"temperature_min_c,omitempty"`
	TemperatureMaxC     *float64          `json:"temperature_max_c,omitempty"`
	PreferredSetpointC  *float64          `json:"preferred_temperature_setpoint_c,omitempty"`
	DangerousGoods      *bool             `json:"dangerous_goods,omitempty"`
	HazardClasses       []string          `json:"hazard_classes,omitempty"`
	OdorEmissionClass   *string           `json:"odor_emission_class,omitempty"`
	OdorSensitive       *bool             `json:"odor_sensitive,omitempty"`
	ContaminationClass  *string           `json:"contamination_class,omitempty"`
	Provenance          map[string]string `json:"provenance,omitempty"`
}

type Equipment struct {
	UnitKind                      *string           `json:"unit_kind,omitempty"`
	CombinationType               *string           `json:"combination_type,omitempty"`
	BodyType                      *string           `json:"body_type,omitempty"`
	EquipmentTypeCode             *string           `json:"equipment_type_code,omitempty"`
	PayloadKg                     *float64          `json:"payload_kg,omitempty"`
	VolumeM3                      *float64          `json:"volume_m3,omitempty"`
	PalletPositions               *int              `json:"pallet_positions,omitempty"`
	PalletBasisCode               *string           `json:"pallet_basis_code,omitempty"`
	UsableLinearMeters            *float64          `json:"usable_linear_meters,omitempty"`
	InternalLengthMM              *int              `json:"internal_length_mm,omitempty"`
	InternalWidthMM               *int              `json:"internal_width_mm,omitempty"`
	InternalHeightMM              *int              `json:"internal_height_mm,omitempty"`
	LoadingAccess                 []string          `json:"loading_access,omitempty"`
	UnloadingAccess               []string          `json:"unloading_access,omitempty"`
	TemperatureControlMode        *string           `json:"temperature_control_mode,omitempty"`
	TemperatureMinC               *float64          `json:"temperature_min_c,omitempty"`
	TemperatureMaxC               *float64          `json:"temperature_max_c,omitempty"`
	TemperatureZoneCount          *int              `json:"temperature_zone_count,omitempty"`
	IndependentTemperatureControl *bool             `json:"independent_temperature_control,omitempty"`
	FoodGradeCapability           *bool             `json:"food_grade_capability,omitempty"`
	ADRCapability                 *bool             `json:"adr_capability,omitempty"`
	Provenance                    map[string]string `json:"provenance,omitempty"`
}

type AccessNeed struct {
	RequiredBodyTypes       []string `json:"required_body_types,omitempty"`
	RequiredEquipmentTypes  []string `json:"required_equipment_types,omitempty"`
	RequiredLoadingAccess   []string `json:"required_loading_access,omitempty"`
	AllowedLoadingAccess    []string `json:"allowed_loading_access,omitempty"`
	RequiredUnloadingAccess []string `json:"required_unloading_access,omitempty"`
	AllowedUnloadingAccess  []string `json:"allowed_unloading_access,omitempty"`
}

type Rule struct {
	RuleCode           string
	RuleKind           string
	Layer              string
	LeftSelectorType   string
	LeftSelectorValue  string
	RightSelectorType  string
	RightSelectorValue string
	Decision           string
	ReasonCode         string
	RequiredSeparation *string
	SourceReference    *string
	Priority           int
	RuleSetVersion     int
}

type Equivalence struct {
	FromCode      string
	BasisCode     string
	PositionsEach float64
}

type CargoClass struct {
	Code   string
	Parent *string
	Tags   []string
	Scope  string
}

type EquipmentClass struct {
	Code                          string
	Scope                         string
	UnitKind                      *string
	CombinationType               *string
	BodyType                      *string
	PayloadKg                     *float64
	VolumeM3                      *float64
	PalletPositions               *int
	UsableLinearMeters            *float64
	InternalLengthMM              *int
	InternalWidthMM               *int
	InternalHeightMM              *int
	LoadingAccess                 []string
	UnloadingAccess               []string
	TemperatureControlMode        *string
	TemperatureMinC               *float64
	TemperatureMaxC               *float64
	TemperatureZoneCount          *int
	IndependentTemperatureControl *bool
	FoodGradeCapability           *bool
	ADRCapability                 *bool
}

type Context struct {
	CargoCatalogVersion     int
	EquipmentCatalogVersion int
	PalletCatalogVersion    int
	PackagingCatalogVersion int
	Rules                   []Rule
	Equivalences            []Equivalence
	CargoClasses            []CargoClass
	EquipmentClasses        []EquipmentClass
	Aliases                 map[string]string
	CatalogInvalid          bool
}

type Reason struct {
	ReasonCode      string  `json:"reason_code"`
	Dimension       string  `json:"dimension"`
	RuleCode        *string `json:"rule_code,omitempty"`
	RuleSetVersion  *int    `json:"rule_set_version,omitempty"`
	CatalogVersion  *int    `json:"catalog_version,omitempty"`
	SourceReference *string `json:"source_reference,omitempty"`
}

type PairResult struct {
	Left    string   `json:"left"`
	Right   string   `json:"right"`
	Status  string   `json:"status"`
	Reasons []Reason `json:"reasons,omitempty"`
}

type Usage struct {
	WeightUsed            *float64 `json:"weight_used"`
	WeightAvailable       *float64 `json:"weight_available"`
	VolumeUsed            *float64 `json:"volume_used"`
	VolumeAvailable       *float64 `json:"volume_available"`
	PalletsUsed           *float64 `json:"pallets_used"`
	PalletsAvailable      *int     `json:"pallets_available"`
	LinearMetersUsed      *float64 `json:"linear_meters_used"`
	LinearMetersAvailable *float64 `json:"linear_meters_available"`
}

type TemperatureOutcome struct {
	CommonMinC *float64 `json:"common_min_c,omitempty"`
	CommonMaxC *float64 `json:"common_max_c,omitempty"`
}

type Result struct {
	Status               string              `json:"status"`
	HardRejects          []Reason            `json:"hard_rejects"`
	IndeterminateReasons []Reason            `json:"indeterminate_reasons"`
	Conditions           []Reason            `json:"conditions"`
	Warnings             []Reason            `json:"warnings"`
	CargoEquipment       []PairResult        `json:"cargo_equipment_results,omitempty"`
	CargoPairs           []PairResult        `json:"cargo_pair_results,omitempty"`
	CapacityUsage        *Usage              `json:"capacity_usage,omitempty"`
	Temperature          *TemperatureOutcome `json:"temperature,omitempty"`
	RuleSetVersions      []int               `json:"rule_set_versions"`
	CatalogVersions      map[string]int      `json:"catalog_versions"`
	Fingerprint          string              `json:"fingerprint"`
}
