package handlers

import (
	"strings"

	db "ifritah/web-service-gin/pkg/db/gen"
)

type zatcaFieldError struct {
	Field  string
	Reason string
}

type zatcaValidationErrors []zatcaFieldError

func (e zatcaValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}

	parts := make([]string, 0, len(e))
	for _, fieldError := range e {
		parts = append(parts, fieldError.Field+" "+fieldError.Reason)
	}
	return "invalid ZATCA onboarding request: " + strings.Join(parts, "; ")
}

func validateZatcaOTP(raw string) (string, error) {
	otp := strings.TrimSpace(raw)
	if len(otp) != 6 || !isASCIIDigits(otp) {
		return "", zatcaValidationErrors{
			{Field: "otp", Reason: "must be exactly 6 ASCII digits"},
		}
	}
	return otp, nil
}

func validateZatcaBranchConfig(config db.GetZatcaBranchConfigRow) error {
	fields := []zatcaFieldError{}
	required := []struct {
		field string
		value string
	}{
		{field: "csr_org_identifier", value: config.Orgid},
		{field: "csr_org_unit", value: config.Orgunit},
		{field: "csr_org_name", value: config.Orgname},
		{field: "csr_country", value: config.Csrcountry},
		{field: "csr_location", value: config.Csrloc},
		{field: "business_category", value: config.Bizcat},
		{field: "seller_vat", value: config.Vat},
		{field: "seller_crn", value: config.Crn},
		{field: "street", value: config.Street},
		{field: "building", value: config.Building},
		{field: "district", value: config.District},
		{field: "postal_code", value: config.Postal},
	}

	trimmed := make(map[string]string, len(required))
	for _, field := range required {
		value := strings.TrimSpace(field.value)
		trimmed[field.field] = value
		if value == "" {
			fields = append(fields, zatcaFieldError{
				Field:  field.field,
				Reason: "is required",
			})
		}
	}

	if trimmed["csr_location"] != "" && !isZatcaLocation(trimmed["csr_location"]) {
		fields = append(fields, zatcaFieldError{
			Field:  "csr_location",
			Reason: "must be 4 uppercase ASCII letters followed by 4 ASCII digits",
		})
	}
	if trimmed["seller_vat"] != "" && !isZatcaVAT(trimmed["seller_vat"]) {
		fields = append(fields, zatcaFieldError{
			Field:  "seller_vat",
			Reason: "must be 15 ASCII digits starting and ending with 3",
		})
	}
	if trimmed["seller_crn"] != "" && !isZatcaCRN(trimmed["seller_crn"]) {
		fields = append(fields, zatcaFieldError{
			Field:  "seller_crn",
			Reason: "must be 10 ASCII digits starting with 7",
		})
	}

	if len(fields) == 0 {
		return nil
	}
	return zatcaValidationErrors(fields)
}

func isZatcaLocation(value string) bool {
	if len(value) != 8 {
		return false
	}
	for i := 0; i < 4; i++ {
		if value[i] < 'A' || value[i] > 'Z' {
			return false
		}
	}
	return isASCIIDigits(value[4:])
}

func isZatcaVAT(value string) bool {
	return len(value) == 15 &&
		value[0] == '3' &&
		value[14] == '3' &&
		isASCIIDigits(value)
}

func isZatcaCRN(value string) bool {
	return len(value) == 10 &&
		value[0] == '7' &&
		isASCIIDigits(value)
}

func isASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
