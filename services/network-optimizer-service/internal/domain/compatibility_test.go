package domain

import "testing"

func TestBNO59ThroughBNO77CapabilityPrimitives(t *testing.T) {
	tent, container, isothermal, refrigerator := BodyTent, BodyContainer, BodyIsothermal, BodyRefrigerator
	if got, err := CanonicalBody(&tent, nil); err != nil || got == nil || *got != BodyTent {
		t.Fatalf("BNO59 got %v err %v", got, err)
	}
	if got, err := CanonicalBody(&container, nil); err != nil || got == nil || *got != BodyContainer {
		t.Fatalf("BNO60 got %v err %v", got, err)
	}
	if got, err := CanonicalBody(&isothermal, nil); err != nil || got == nil || *got != BodyIsothermal {
		t.Fatalf("BNO61 got %v err %v", got, err)
	}
	if got, err := CanonicalBody(&refrigerator, nil); err != nil || got == nil || *got != BodyRefrigerator {
		t.Fatalf("BNO62 got %v err %v", got, err)
	}
	rear, err := NormalizeAccess([]string{AccessRear})
	if err != nil || len(rear) != 1 || rear[0] != AccessRear {
		t.Fatalf("BNO63 %v %v", rear, err)
	}
	side, err := NormalizeAccess([]string{AccessSide})
	if err != nil || side[0] != AccessSide {
		t.Fatalf("BNO64 %v", side)
	}
	top, err := NormalizeAccess([]string{AccessTop})
	if err != nil || top[0] != AccessTop {
		t.Fatalf("BNO65 %v", top)
	}
	loading := []string{AccessRear, AccessSide, AccessTop}
	unloading := []string{AccessRear, AccessSide}
	if LoadingCompatible([]string{AccessTop}, loading).Status != ResultCompatible {
		t.Fatal("BNO66 loading")
	}
	if UnloadingCompatible([]string{AccessTop}, unloading).Status != ResultIncompatible {
		t.Fatal("BNO66 unloading must stay independent")
	}
	if LoadingCompatible([]string{AccessSide}, nil).Status != ResultIndeterminate || LoadingCompatible([]string{AccessSide}, nil).Reason != ReasonLoadingAccessUnknown {
		t.Fatal("BNO67 unknown access must not be false")
	}
	if LoadingCompatible([]string{AccessSide}, []string{}).Status != ResultIncompatible {
		t.Fatal("BNO67 known empty access is unsupported")
	}
	legacy := "curtain sider"
	if got, err := CanonicalBody(nil, &legacy); err != nil || got != nil {
		t.Fatalf("BNO68/BNO69 invented body %v %v", got, err)
	}
	exact := "TENT"
	if got, err := CanonicalBody(nil, &exact); err != nil || got == nil || *got != BodyTent {
		t.Fatalf("exact legacy token %v %v", got, err)
	}

	active := TemperatureActive
	passive := TemperaturePassive
	min, max := -25.0, 20.0
	chilledMin, chilledMax := 2.0, 8.0
	if SingleCargoTemperature(TemperatureCapability{ControlMode: &active, MinC: &min, MaxC: &max}, TemperatureRequirement{Required: true, MinC: &chilledMin, MaxC: &chilledMax}).Status != ResultCompatible {
		t.Fatal("BNO71")
	}
	zero := 0.0
	if result := SingleCargoTemperature(TemperatureCapability{ControlMode: &active, MinC: &min, MaxC: &zero}, TemperatureRequirement{Required: true, MinC: &chilledMin, MaxC: &chilledMax}); result.Status != ResultIncompatible || result.Reason != ReasonTemperatureRangeUnsupported {
		t.Fatalf("BNO72 %+v", result)
	}
	if result := SingleCargoTemperature(TemperatureCapability{ControlMode: &active}, TemperatureRequirement{Required: true, MinC: &chilledMin, MaxC: &chilledMax}); result.Status != ResultIndeterminate || result.Reason != ReasonTemperatureCapabilityUnknown {
		t.Fatalf("BNO73 %+v", result)
	}
	if result := SingleCargoTemperature(TemperatureCapability{ControlMode: &passive, MinC: &min, MaxC: &max}, TemperatureRequirement{Required: true, MinC: &chilledMin, MaxC: &chilledMax}); result.Status != ResultIncompatible || result.Reason != ReasonTemperatureControlRequired {
		t.Fatalf("BNO74 %+v", result)
	}
	frozenMin, frozenMax := -25.0, -18.0
	if result := SingleZoneCoLoad(&chilledMin, &chilledMax, &frozenMin, &frozenMax); result.Status != ResultIncompatible || result.Reason != ReasonTemperatureRangesIncompatible {
		t.Fatalf("BNO75 %+v", result)
	}
	overlapMin, overlapMax := 4.0, 6.0
	if SingleZoneCoLoad(&chilledMin, &chilledMax, &overlapMin, &overlapMax).Status != ResultCompatible {
		t.Fatal("BNO76")
	}
	zones := 2
	if result := MultiZoneOptimization(&zones); result.Status != ResultIndeterminate || result.Reason != ReasonMultiZoneRequired {
		t.Fatalf("BNO77 %+v", result)
	}
	if result := MultiZoneOptimization(nil); result.Status == ResultCompatible {
		t.Fatal("BNO77 nil zone count must not become capability")
	}
	setpoint := -20.0
	if TemperatureTransition(&setpoint, &chilledMin, &chilledMax, true).Reason != ReasonTemperaturePreconditioning {
		t.Fatal("setpoint outside the next interval must not claim immediate readiness")
	}
}
