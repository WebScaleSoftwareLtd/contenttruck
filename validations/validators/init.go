package validators

// Validator is used to define the interface for a validator.
type Validator interface {
	// Matches is used to check if a validator matches a string.
	Matches(s string) bool

	// Validate is used to validate a byte slice.
	Validate(b []byte, _ string) error
}

// Validators is used to define a list of validators in this package.
var Validators []Validator
