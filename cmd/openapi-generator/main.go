// Package main provides the CLI for the OpenAPI Generator Go implementation.
package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xseman/openapi-generator/internal/gen"
	"github.com/xseman/openapi-generator/internal/update"
	"gopkg.in/yaml.v3"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "openapi-generator",
	Short: "OpenAPI Generator - Generate API clients from OpenAPI specs",
	Long: `OpenAPI Generator is a Go implementation of the OpenAPI Generator.
It generates TypeScript Fetch API clients from OpenAPI 3.x specifications.

This tool is compatible with the Java-based openapi-generator and uses
the same Mustache templates for code generation.`,
	Version: update.Version,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate code from an OpenAPI specification",
	Long: `Generate client code from an OpenAPI specification file.

Example:
  openapi-generator generate -i petstore.yaml -g typescript-fetch -o ./generated`,
	RunE: runGenerate,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate an OpenAPI specification",
	Long: `Validate an OpenAPI specification without generating code.

Errors are reported to stderr and the command exits with status 1.
With --recommend, non-fatal recommendations (such as unused models) are
listed as well.

Example:
  openapi-generator validate -i petstore.yaml --recommend`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runValidate,
}

var (
	inputSpec            string
	outputDir            string
	generatorName        string
	configFile           string
	templateDir          string
	additionalProperties []string
	skipValidation       bool
	verbose              bool
	recommend            bool
)

func init() {
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configHelpCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(updateCmd)

	// Generate command flags
	generateCmd.Flags().StringVarP(&inputSpec, "input-spec", "i", "", "OpenAPI spec file")
	generateCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory")
	generateCmd.Flags().StringVarP(&generatorName, "generator-name", "g", "", "Generator to use")
	generateCmd.Flags().StringVarP(&configFile, "config", "c", "", "Configuration file (JSON/YAML)")
	generateCmd.Flags().StringVarP(&templateDir, "template-dir", "t", "", "Custom template directory")
	generateCmd.Flags().StringArrayVarP(&additionalProperties, "additional-properties", "p", nil, "Key=value")
	generateCmd.Flags().BoolVar(&skipValidation, "skip-validate-spec", false, "Skip spec validation")
	generateCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Validate command flags
	validateCmd.Flags().StringVarP(&inputSpec, "input-spec", "i", "", "OpenAPI spec file or URL (required)")
	validateCmd.Flags().BoolVar(&recommend, "recommend", false, "Also list recommendations (e.g. unused models)")
	_ = validateCmd.MarkFlagRequired("input-spec")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available generators",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("Available generators:")
		fmt.Println()
		fmt.Println("CLIENT generators:")
		fmt.Println("  - typescript-fetch")
		fmt.Println("  - dart-fetch")
		fmt.Println()
	},
}

var configHelpCmd = &cobra.Command{
	Use:   "config-help",
	Short: "Show configuration options for a generator",
	Run: func(_ *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Usage: openapi-generator config-help <generator-name>")
			return
		}

		switch args[0] {
		case "typescript-fetch":
			gen.PrintTypeScriptFetchConfigHelp()
		case "dart-fetch":
			gen.PrintDartFetchConfigHelp()
		default:
			fmt.Printf("Unknown generator: %s\n", args[0])
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("openapi-generator %s\n", update.Version)
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Install the latest release over this binary",
	Long: `Check GitHub for a newer release and replace this executable with it.
The download is verified against the release's CHECKSUMS.txt before anything
is replaced; the new binary runs from the next start.`,
	SilenceUsage: true,
	RunE:         runUpdate,
}

// runUpdate replaces this binary with the latest release. A build without a
// version (a local go build) installs the latest release as well: it is the
// way back from a working copy to a released openapi-generator.
func runUpdate(cmd *cobra.Command, _ []string) error {
	rel, err := update.Check(cmd.Context())
	if err != nil {
		return err
	}

	if !rel.Newer() && update.Version != update.Dev {
		fmt.Printf("openapi-generator %s is the latest release\n", update.Version)
		return nil
	}

	fmt.Printf("openapi-generator %s → %s\n", update.Version, rel.Version)

	progress := func(done, total int64) {
		if total > 0 {
			fmt.Fprintf(os.Stderr, "\r%3d%%", done*100/total)
		}
	}
	if err := update.Install(cmd.Context(), rel, progress); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Printf("installed openapi-generator %s, it runs from the next start\n", rel.Version)

	return nil
}

// runValidate validates the spec given via --input-spec and prints a
// report in the same format as the Java openapi-generator's validate
// command. Errors go to stderr and make the process exit with status 1.
func runValidate(_ *cobra.Command, _ []string) error {
	fmt.Printf("Validating spec (%s)\n", inputSpec)

	res := gen.Validate(inputSpec)

	report, ok := formatValidationReport(res, recommend)
	if !ok {
		fmt.Fprint(os.Stderr, report)
		os.Exit(1)
	}

	fmt.Print(report)

	return nil
}

// formatValidationReport renders a ValidationResult the way the Java CLI
// does: an optional "Warnings:" block (only when recommend is set), an
// "Errors:" block, and a summary line. ok is false when the spec has
// errors.
func formatValidationReport(res gen.ValidationResult, recommend bool) (report string, ok bool) {
	var sb strings.Builder

	warnings := res.Warnings
	if !recommend {
		warnings = nil
	}

	if len(warnings) > 0 {
		sb.WriteString("Warnings:\n")

		for _, msg := range warnings {
			fmt.Fprintf(&sb, "\t- %s\n", msg)
		}
	}

	switch {
	case len(res.Errors) > 0:
		sb.WriteString("Errors:\n")

		for _, msg := range res.Errors {
			fmt.Fprintf(&sb, "\t- %s\n", msg)
		}

		fmt.Fprintf(&sb, "[error] Spec has %d errors.\n", len(res.Errors))

		return sb.String(), false

	case len(warnings) > 0:
		fmt.Fprintf(&sb, "[info] Spec has %d recommendation(s).\n", len(warnings))
	default:
		sb.WriteString("No validation issues detected.\n")
	}

	return sb.String(), true
}

// Config represents the configuration file structure.
// It mirrors the Java openapi-generator config format.
type Config struct {
	GeneratorName        string            `json:"generatorName" yaml:"generatorName"`
	InputSpec            string            `json:"inputSpec" yaml:"inputSpec"`
	OutputDir            string            `json:"outputDir" yaml:"outputDir"`
	TemplateDir          string            `json:"templateDir" yaml:"templateDir"`
	AdditionalProperties map[string]string `json:"additionalProperties" yaml:"additionalProperties"`
	SkipValidation       bool              `json:"skipValidateSpec" yaml:"skipValidateSpec"`
	Verbose              bool              `json:"verbose" yaml:"verbose"`
}

// loadConfigFile loads configuration from a JSON or YAML file.
func loadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config

	// Determine format based on file extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}

	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}

	default:
		// Try JSON first, then YAML
		if err := json.Unmarshal(data, &cfg); err != nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file (tried JSON and YAML): %w", err)
			}
		}
	}

	return &cfg, nil
}

// runGenerate resolves the effective options from CLI flags and an optional
// config file (CLI flags take precedence), then runs the generation
// pipeline.
func runGenerate(_ *cobra.Command, _ []string) error {
	if configFile != "" {
		cfg, err := loadConfigFile(configFile)
		if err != nil {
			return err
		}

		applyConfig(cfg)
	}

	return gen.Generate(gen.Options{
		InputSpec:            inputSpec,
		OutputDir:            outputDir,
		GeneratorName:        generatorName,
		TemplateDir:          templateDir,
		AdditionalProperties: additionalProperties,
		SkipValidation:       skipValidation,
		Verbose:              verbose,
		Version:              update.Version,
	})
}

// applyConfig fills every option the flags left empty from the config file;
// a flag always wins. The file's additional properties join the flags' in
// key order, so the merged list is stable across runs.
func applyConfig(cfg *Config) {
	if inputSpec == "" {
		inputSpec = cfg.InputSpec
	}

	if outputDir == "" {
		outputDir = cfg.OutputDir
	}

	if generatorName == "" {
		generatorName = cfg.GeneratorName
	}

	if templateDir == "" {
		templateDir = cfg.TemplateDir
	}

	skipValidation = skipValidation || cfg.SkipValidation
	verbose = verbose || cfg.Verbose

	for _, k := range slices.Sorted(maps.Keys(cfg.AdditionalProperties)) {
		given := func(prop string) bool { return strings.HasPrefix(prop, k+"=") }
		if !slices.ContainsFunc(additionalProperties, given) {
			additionalProperties = append(additionalProperties, k+"="+cfg.AdditionalProperties[k])
		}
	}
}
