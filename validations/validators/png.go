package validators

import (
	"errors"
	"image/png"
	"strings"
)

type pngValidator struct{}

var _ Validator = (*pngValidator)(nil)

// Matches is used to check if a validator matches a string.
func (p *pngValidator) Matches(s string) bool {
	return s == "png"
}

// Validate is used to validate a byte slice is a png.
func (p *pngValidator) Validate(b []byte, _ string) error {
	_, err := png.Decode(strings.NewReader(string(b)))
	if err != nil {
		return errors.New("The image specified is not a png")
	}
	return nil
}

func init() {
	Validators = append(Validators, &pngValidator{})
}
