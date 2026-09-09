package gen

import (
	"github.com/xseman/openapi-generator/internal/parser"
)

// ValidationResult holds the outcome of validating an OpenAPI document.
type ValidationResult struct {
	// Errors are problems that make the spec invalid: it could not be
	// loaded, or kin-openapi validation rejected it.
	Errors []string
	// Warnings are recommendations that do not invalidate the spec, such
	// as models declared under components/schemas but never referenced.
	Warnings []string
}

// Validate loads inputSpec (a local file path or an http(s) URL) and
// returns its validation errors and warnings. A spec that cannot be
// loaded at all is reported as a single error rather than a failure, so
// callers can treat every outcome uniformly.
func Validate(inputSpec string) ValidationResult {
	p := parser.NewParser()
	// Collect issues instead of failing on them; reporting is up to the
	// caller.
	p.SkipValidation = true

	if err := loadSpec(p, inputSpec); err != nil {
		return ValidationResult{Errors: []string{err.Error()}}
	}

	return ValidationResult{
		Errors:   p.ValidationErrors,
		Warnings: p.ValidationWarnings,
	}
}
