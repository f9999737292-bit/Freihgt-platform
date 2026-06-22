package domain

import "testing"

func TestValidateDraftFormTemplateInput(t *testing.T) {
	valid := DraftFormTemplateInput{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_custom_v1",
		Name:       "Custom Form",
		Sections: []DraftFormSectionInput{
			{
				Code:      "cargo",
				Title:     "Cargo",
				SortOrder: 100,
				Fields: []DraftFormFieldInput{
					{
						Code:      "cargo_class",
						Label:     "Cargo class",
						FieldType: "SELECT",
						SortOrder: 100,
						OptionsJSON: []byte(`{"options":["GENERAL","DANGEROUS"]}`),
					},
				},
			},
		},
	}
	if err := ValidateDraftFormTemplateInput(valid); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
}

func TestValidateDraftFormTemplateInputInvalidEntityType(t *testing.T) {
	err := ValidateDraftFormTemplateInput(DraftFormTemplateInput{
		EntityType: "BAD",
		Code:       "transport_order_custom_v1",
		Name:       "Custom Form",
		Sections: []DraftFormSectionInput{
			{Code: "cargo", Title: "Cargo", Fields: []DraftFormFieldInput{
				{Code: "cargo_class", Label: "Cargo class", FieldType: "TEXT"},
			}},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateDraftFormTemplateInputInvalidFieldType(t *testing.T) {
	err := ValidateDraftFormTemplateInput(DraftFormTemplateInput{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_custom_v1",
		Name:       "Custom Form",
		Sections: []DraftFormSectionInput{
			{Code: "cargo", Title: "Cargo", Fields: []DraftFormFieldInput{
				{Code: "cargo_class", Label: "Cargo class", FieldType: "BAD"},
			}},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateDraftFormTemplateInputSQLFragment(t *testing.T) {
	err := ValidateDraftFormTemplateInput(DraftFormTemplateInput{
		EntityType: "TRANSPORT_ORDER",
		Code:       "transport_order_custom_v1",
		Name:       "Custom Form",
		Sections: []DraftFormSectionInput{
			{Code: "cargo", Title: "Cargo", Fields: []DraftFormFieldInput{
				{
					Code:               "cargo_class",
					Label:              "Cargo class",
					FieldType:          "TEXT",
					ValidationRuleJSON: []byte(`{"rule":"SELECT * FROM users"}`),
				},
			}},
		},
	})
	if err == nil {
		t.Fatal("expected validation error for SQL fragment")
	}
}
