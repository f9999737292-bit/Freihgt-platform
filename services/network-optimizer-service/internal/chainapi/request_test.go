package chainapi

import "testing"

func TestI2C_POST_03_predicted_capacity_rejected(t *testing.T) {
	_, err := parsePublicSeed([]byte(`{"seed_type":"PREDICTED_CAPACITY","capacity_id":"11111111-1111-1111-1111-111111111111"}`))
	status, ok := err.(*StatusError)
	if !ok || status.Status != 422 {
		t.Fatalf("status=%v err=%v", status, err)
	}
}

func TestI2C_POST_04_candidate_load_ids_rejected(t *testing.T) {
	_, err := parsePublicSeed([]byte(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"11111111-1111-1111-1111-111111111111","candidate_load_ids":[]}`))
	status, ok := err.(*StatusError)
	if !ok || status.Status != 422 {
		t.Fatalf("status=%v", status)
	}
}

func TestI2C_POST_05_client_l1_rejected(t *testing.T) {
	_, err := parsePublicSeed([]byte(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"11111111-1111-1111-1111-111111111111","l1":"x"}`))
	status, ok := err.(*StatusError)
	if !ok || status.Status != 422 {
		t.Fatalf("status=%v", status)
	}
}

func TestI2C_POST_06_client_chain_search_id_rejected(t *testing.T) {
	_, err := parsePublicSeed([]byte(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"11111111-1111-1111-1111-111111111111","chain_search_id":"11111111-1111-1111-1111-111111111111"}`))
	status, ok := err.(*StatusError)
	if !ok || status.Status != 422 {
		t.Fatalf("status=%v", status)
	}
}

func TestI2C_malformed_json_is_400(t *testing.T) {
	_, err := parsePublicSeed([]byte(`{`))
	status, ok := err.(*StatusError)
	if !ok || status.Status != 400 {
		t.Fatalf("status=%v", status)
	}
}

func TestI2C_JSON_01_single_object_accepted(t *testing.T) {
	seed, err := parsePublicSeed([]byte(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"11111111-1111-1111-1111-111111111111"}`))
	if err != nil || seed.SeedType != PublicSeedManualCapacity {
		t.Fatalf("seed=%+v err=%v", seed, err)
	}
}

func TestI2C_JSON_02_second_object_rejected(t *testing.T) {
	_, err := parsePublicSeed([]byte(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"11111111-1111-1111-1111-111111111111"}{"seed_type":"CURRENT_TRIP"}`))
	status, ok := err.(*StatusError)
	if !ok || status.Status != 400 || status.Code != "VALIDATION_ERROR" {
		t.Fatalf("status=%v err=%v", status, err)
	}
}

func TestI2C_JSON_03_trailing_garbage_rejected(t *testing.T) {
	_, err := parsePublicSeed([]byte(`{"seed_type":"MANUAL_CAPACITY","capacity_id":"11111111-1111-1111-1111-111111111111"} garbage`))
	status, ok := err.(*StatusError)
	if !ok || status.Status != 400 {
		t.Fatalf("status=%v err=%v", status, err)
	}
}

func TestI2C_JSON_04_malformed_object_rejected(t *testing.T) {
	_, err := parsePublicSeed([]byte(`{"seed_type":`))
	status, ok := err.(*StatusError)
	if !ok || status.Status != 400 {
		t.Fatalf("status=%v err=%v", status, err)
	}
}

func TestI2C_JSON_05_trailing_whitespace_accepted(t *testing.T) {
	_, err := parsePublicSeed([]byte("{\"seed_type\":\"MANUAL_CAPACITY\",\"capacity_id\":\"11111111-1111-1111-1111-111111111111\"}\n\t "))
	if err != nil {
		t.Fatal(err)
	}
}

func TestI2C_omitted_key_is_server_owned(t *testing.T) {
	a, _, err := parseIdempotencyKey("", false)
	b, _, err2 := parseIdempotencyKey("", false)
	if err != nil || err2 != nil || a == "" || a == b || len(a) > 128 {
		t.Fatalf("a=%s b=%s", a, b)
	}
}
