package validations

import (
	"io"
	"strings"

	"contenttruck/validations/validators"
)

// Execute is used to execute the validations.
func Execute(r io.Reader, validations string) (io.Reader, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	for _, v := range strings.Split(validations, "+") {
		for _, validator := range validators.Validators {
			if validator.Matches(v) {
				err = validator.Validate(b, v)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return strings.NewReader(string(b)), nil
}
