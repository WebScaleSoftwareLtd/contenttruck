package validators

import (
	"encoding/xml"
	"errors"
)

type svgValidator struct{}

var _ Validator = (*svgValidator)(nil)

// Matches is used to check if a validator matches a string.
func (p *svgValidator) Matches(s string) bool {
	return s == "svg"
}

// Validate is used to validate a byte slice is a svg.
func (p *svgValidator) Validate(b []byte, _ string) error {
	var doc struct {
		XMLName xml.Name `xml:"svg"`
	}
	if err := xml.Unmarshal(b, &doc); err != nil {
		return errors.New("The image specified is not a svg")
	}
	return nil
}

func init() {
	Validators = append(Validators, &svgValidator{})
}
