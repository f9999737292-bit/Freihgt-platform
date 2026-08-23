package domain

import "testing"

func TestFC22CLane001MoscowToSPB(t *testing.T) {
	result := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", OriginCity: "Moscow",
		DestinationCountry: "RU", DestinationCity: "SPB",
		TransportMode: "ROAD", EquipmentType: "TENT",
	})
	if !result.Available {
		t.Fatalf("expected available lane, got %s", result.ExclusionReason)
	}
	want := "RU:MOSCOW->RU:SPB|ROAD|TENT"
	if result.LaneKey != want {
		t.Fatalf("expected %q, got %q", want, result.LaneKey)
	}
}

func TestFC22CLane002WhitespaceNormalized(t *testing.T) {
	result := BuildLaneKey(LaneKeyInput{
		OriginCountry: " ru ", OriginCity: "  Moscow ",
		DestinationCountry: "RU", DestinationCity: "SPB",
		TransportMode: " ROAD ", EquipmentType: " TENT ",
	})
	if result.OriginCity != "MOSCOW" || result.TransportMode != "ROAD" {
		t.Fatalf("unexpected normalization: %+v", result)
	}
}

func TestFC22CLane003CaseNormalized(t *testing.T) {
	result := BuildLaneKey(LaneKeyInput{
		OriginCountry: "ru", OriginCity: "moscow",
		DestinationCountry: "ru", DestinationCity: "spb",
		TransportMode: "road", EquipmentType: "tent",
	})
	if result.LaneKey != "RU:MOSCOW->RU:SPB|ROAD|TENT" {
		t.Fatalf("unexpected key: %s", result.LaneKey)
	}
}

func TestFC22CLane004Directional(t *testing.T) {
	ab := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", OriginCity: "MOSCOW",
		DestinationCountry: "RU", DestinationCity: "SPB",
		TransportMode: "ROAD", EquipmentType: "TENT",
	})
	ba := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", OriginCity: "SPB",
		DestinationCountry: "RU", DestinationCity: "MOSCOW",
		TransportMode: "ROAD", EquipmentType: "TENT",
	})
	if ab.LaneKey == ba.LaneKey {
		t.Fatal("directional lanes must differ")
	}
}

func TestFC22CLane005EquipmentWild(t *testing.T) {
	result := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", OriginCity: "MOSCOW",
		DestinationCountry: "RU", DestinationCity: "SPB",
		TransportMode: "ROAD",
	})
	if result.EquipmentType != EquipmentTypeWild {
		t.Fatalf("expected WILD equipment, got %s", result.EquipmentType)
	}
	if !stringsHasSuffix(result.LaneKey, "|ROAD|WILD") {
		t.Fatalf("unexpected key: %s", result.LaneKey)
	}
}

func TestFC22CLane006MissingOriginCity(t *testing.T) {
	result := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", DestinationCountry: "RU", DestinationCity: "SPB",
		TransportMode: "ROAD",
	})
	if result.Available || result.ExclusionReason != LaneExclusionMissingOriginCity {
		t.Fatalf("expected missing origin city, got %+v", result)
	}
}

func TestFC22CLane007MissingDestinationCity(t *testing.T) {
	result := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", OriginCity: "MOSCOW", DestinationCountry: "RU",
		TransportMode: "ROAD",
	})
	if result.Available || result.ExclusionReason != LaneExclusionMissingDestinationCity {
		t.Fatalf("expected missing destination city, got %+v", result)
	}
}

func TestFC22CLane008MissingCountry(t *testing.T) {
	result := BuildLaneKey(LaneKeyInput{
		OriginCity: "MOSCOW", DestinationCountry: "RU", DestinationCity: "SPB",
		TransportMode: "ROAD",
	})
	if result.Available || result.ExclusionReason != LaneExclusionMissingOriginCountry {
		t.Fatalf("expected missing origin country, got %+v", result)
	}
}

func TestFC22CLane009MissingMode(t *testing.T) {
	result := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", OriginCity: "MOSCOW",
		DestinationCountry: "RU", DestinationCity: "SPB",
	})
	if result.Available || result.ExclusionReason != LaneExclusionMissingTransportMode {
		t.Fatalf("expected missing mode, got %+v", result)
	}
}

func TestFC22CLane010UnicodeCityDeterministic(t *testing.T) {
	first := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", OriginCity: "Санкт-Петербург",
		DestinationCountry: "RU", DestinationCity: "Москва",
		TransportMode: "ROAD", EquipmentType: "TENT",
	})
	second := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", OriginCity: "Санкт-Петербург",
		DestinationCountry: "RU", DestinationCity: "Москва",
		TransportMode: "ROAD", EquipmentType: "TENT",
	})
	if first.LaneKey != second.LaneKey || !first.Available {
		t.Fatalf("unicode lane key not deterministic: %+v vs %+v", first, second)
	}
}

func TestFC22CLane011LocationUUIDNotUsed(t *testing.T) {
	uuidLike := "550e8400-e29b-41d4-a716-446655440000"
	result := BuildLaneKey(LaneKeyInput{
		OriginCountry: "RU", OriginCity: uuidLike,
		DestinationCountry: "RU", DestinationCity: "SPB",
		TransportMode: "ROAD",
	})
	if result.LaneKey != "RU:"+stringsToUpperNoSpace(uuidLike)+"->RU:SPB|ROAD|WILD" {
		t.Fatalf("uuid-like city is normalized text, not special-cased: %s", result.LaneKey)
	}
}

func TestFC22CLane012SameInputsSameKey(t *testing.T) {
	in := LaneKeyInput{
		OriginCountry: "RU", OriginCity: "MOSCOW",
		DestinationCountry: "RU", DestinationCity: "KAZAN",
		TransportMode: "ROAD", EquipmentType: "TENT",
	}
	if BuildLaneKey(in).LaneKey != BuildLaneKey(in).LaneKey {
		t.Fatal("lane key must be deterministic")
	}
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func stringsToUpperNoSpace(s string) string {
	result := BuildLaneKey(LaneKeyInput{OriginCountry: "RU", OriginCity: s, DestinationCountry: "RU", DestinationCity: "SPB", TransportMode: "ROAD"})
	return result.OriginCity
}
