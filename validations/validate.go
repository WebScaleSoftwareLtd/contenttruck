package validations

import (
	"strings"

	"contenttruck/validations/validators"
)

// Validate is used to validate the validations string.
func Validate(validations string) bool {
	for _, v := range strings.Split(validations, "+") {
		match := false
		for _, validator := range validators.Validators {
			if validator.Matches(v) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}
