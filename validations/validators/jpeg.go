package validators

import (
	"errors"
	"image/jpeg"
	"strings"
)

type jpegValidator struct{}

var _ Validator = (*jpegValidator)(nil)

// Matches is used to check if a validator matches a string.
func (p *jpegValidator) Matches(s string) bool {
	return s == "jpeg" || s == "jpg"
}

// Validate is used to validate a byte slice is a jpeg.
func (p *jpegValidator) Validate(b []byte, _ string) error {
	_, err := jpeg.Decode(strings.NewReader(string(b)))
	if err != nil {
		return errors.New("The image specified is not a jpeg")
	}
	return nil
}

func init() {
	Validators = append(Validators, &jpegValidator{})
}
