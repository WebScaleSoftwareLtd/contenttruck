package validators

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"github.com/disintegration/imaging"
)

type aspectRatioValidator struct{}

var _ Validator = (*aspectRatioValidator)(nil)

var aspectRatioRegex = regexp.MustCompile(`^(\d+):(\d+)$`)

// Matches is used to check if a validator matches a string.
func (p *aspectRatioValidator) Matches(s string) bool {
	return aspectRatioRegex.MatchString(s)
}

// Calculates aspect ratio from a width and height. Returns a string in the format of "X:Y".
func calculateAspectRatio(width, height int) string {
	if width == 0 || height == 0 {
		return "0:0"
	}
	gcd := func(a, b int) int {
		for b != 0 {
			a, b = b, a%b
		}
		return a
	}
	divisor := gcd(width, height)
	aspectRatioWidth := width / divisor
	aspectRatioHeight := height / divisor
	return fmt.Sprintf("%d:%d", aspectRatioWidth, aspectRatioHeight)
}

// Validate is used to validate a byte slice is the aspect ratio specified.
func (p *aspectRatioValidator) Validate(b []byte, s string) error {
	img, err := imaging.Decode(bytes.NewReader(b))
	if err != nil {
		return errors.New("The image specified is not a valid image")
	}
	bounds := img.Bounds()

	// Calculate the aspect ratio.
	aspectRatio := calculateAspectRatio(bounds.Dx(), bounds.Dy())

	// Check if the aspect ratio matches.
	if aspectRatio != s {
		return errors.New("The image specified does not match the aspect ratio")
	}
	return nil
}

func init() {
	Validators = append(Validators, &aspectRatioValidator{})
}
