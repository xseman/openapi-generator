package main

import (
	"testing"

	"github.com/xseman/openapi-generator/internal/gen"
)

func TestFormatValidationReport(t *testing.T) {
	tests := []struct {
		name      string
		res       gen.ValidationResult
		recommend bool
		want      string
		wantOK    bool
	}{
		{
			name:   "clean spec",
			want:   "No validation issues detected.\n",
			wantOK: true,
		},
		{
			name:   "warnings are hidden without --recommend",
			res:    gen.ValidationResult{Warnings: []string{"Unused model: A"}},
			want:   "No validation issues detected.\n",
			wantOK: true,
		},
		{
			name:      "warnings are listed with --recommend",
			res:       gen.ValidationResult{Warnings: []string{"Unused model: A", "Unused model: B"}},
			recommend: true,
			want:      "Warnings:\n\t- Unused model: A\n\t- Unused model: B\n[info] Spec has 2 recommendation(s).\n",
			wantOK:    true,
		},
		{
			name:   "errors fail the report",
			res:    gen.ValidationResult{Errors: []string{"invalid info: must be an object"}},
			want:   "Errors:\n\t- invalid info: must be an object\n[error] Spec has 1 errors.\n",
			wantOK: false,
		},
		{
			name: "warnings precede errors with --recommend",
			res: gen.ValidationResult{
				Errors:   []string{"boom"},
				Warnings: []string{"Unused model: A"},
			},
			recommend: true,
			want:      "Warnings:\n\t- Unused model: A\nErrors:\n\t- boom\n[error] Spec has 1 errors.\n",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := formatValidationReport(tt.res, tt.recommend)
			if got != tt.want {
				t.Errorf("report = %q, want %q", got, tt.want)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}
