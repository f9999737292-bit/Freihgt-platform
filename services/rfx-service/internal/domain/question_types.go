package domain

import (
	"strings"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	QuestionTypeText            = "TEXT"
	QuestionTypeLongText        = "LONG_TEXT"
	QuestionTypeNumber          = "NUMBER"
	QuestionTypeMoney           = "MONEY"
	QuestionTypeYesNo           = "YES_NO"
	QuestionTypeSingleSelect    = "SINGLE_SELECT"
	QuestionTypeMultiSelect     = "MULTI_SELECT"
	QuestionTypeDate            = "DATE"
	QuestionTypeDateTime        = "DATETIME"
	QuestionTypeFile            = "FILE"
	QuestionTypeTable           = "TABLE"
	QuestionTypeAddress         = "ADDRESS"
	QuestionTypeCountry         = "COUNTRY"
	QuestionTypeCompany         = "COMPANY"
	QuestionTypeVehicleCategory = "VEHICLE_CATEGORY"
	QuestionTypeCertificate     = "CERTIFICATE"
	QuestionTypePercent         = "PERCENT"
	QuestionTypeRating          = "RATING"
)

var allowedQuestionTypes = map[string]struct{}{
	QuestionTypeText: {}, QuestionTypeLongText: {}, QuestionTypeNumber: {},
	QuestionTypeMoney: {}, QuestionTypeYesNo: {}, QuestionTypeSingleSelect: {},
	QuestionTypeMultiSelect: {}, QuestionTypeDate: {}, QuestionTypeDateTime: {},
	QuestionTypeFile: {}, QuestionTypeTable: {}, QuestionTypeAddress: {},
	QuestionTypeCountry: {}, QuestionTypeCompany: {}, QuestionTypeVehicleCategory: {},
	QuestionTypeCertificate: {}, QuestionTypePercent: {}, QuestionTypeRating: {},
}

func ValidateQuestionType(value string) error {
	value = strings.TrimSpace(value)
	if _, ok := allowedQuestionTypes[value]; !ok {
		return apperrors.Validation("invalid question_type", map[string]any{"field": "question_type", "value": value})
	}
	return nil
}

func QuestionTypeRequiresOptions(questionType string) bool {
	switch strings.TrimSpace(questionType) {
	case QuestionTypeSingleSelect, QuestionTypeMultiSelect:
		return true
	default:
		return false
	}
}

func QuestionTypeSupportsNumericValidation(questionType string) bool {
	switch strings.TrimSpace(questionType) {
	case QuestionTypeNumber, QuestionTypeMoney, QuestionTypePercent, QuestionTypeRating:
		return true
	default:
		return false
	}
}

func CanonicalQuestionTypes() []string {
	return []string{
		QuestionTypeText, QuestionTypeLongText, QuestionTypeNumber, QuestionTypeMoney,
		QuestionTypeYesNo, QuestionTypeSingleSelect, QuestionTypeMultiSelect,
		QuestionTypeDate, QuestionTypeDateTime, QuestionTypeFile, QuestionTypeTable,
		QuestionTypeAddress, QuestionTypeCountry, QuestionTypeCompany,
		QuestionTypeVehicleCategory, QuestionTypeCertificate, QuestionTypePercent, QuestionTypeRating,
	}
}
