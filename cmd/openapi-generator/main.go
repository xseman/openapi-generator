// Package main provides the CLI for the OpenAPI Generator Go implementation.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xseman/openapi-generator/internal/gen"
	"gopkg.in/yaml.v3"
)

var (
	// version is set at build time using -ldflags="-X main.version=x.y.z"
	version = "dev"
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
	Version: version,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate code from an OpenAPI specification",
	Long: `Generate client code from an OpenAPI specification file.

Example:
  openapi-generator generate -i petstore.yaml -g typescript-fetch -o ./generated`,
	RunE: runGenerate,
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
)

func init() {
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configHelpCmd)
	rootCmd.AddCommand(versionCmd)

	// Generate command flags
	generateCmd.Flags().StringVarP(&inputSpec, "input-spec", "i", "", "OpenAPI spec file")
	generateCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Output directory")
	generateCmd.Flags().StringVarP(&generatorName, "generator-name", "g", "", "Generator to use")
	generateCmd.Flags().StringVarP(&configFile, "config", "c", "", "Configuration file (JSON/YAML)")
	generateCmd.Flags().StringVarP(&templateDir, "template-dir", "t", "", "Custom template directory")
	generateCmd.Flags().StringArrayVarP(&additionalProperties, "additional-properties", "p", nil, "Key=value")
	generateCmd.Flags().BoolVar(&skipValidation, "skip-validate-spec", false, "Skip spec validation")
	generateCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available generators",
	Run: func(cmd *cobra.Command, args []string) {
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
	Run: func(cmd *cobra.Command, args []string) {
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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("openapi-generator %s\n", version)
	},
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
func runGenerate(cmd *cobra.Command, args []string) error {
	// Load config file if specified
	if configFile != "" {
		cfg, err := loadConfigFile(configFile)
		if err != nil {
			return err
		}

		// Apply config values, CLI flags override config file
		if inputSpec == "" && cfg.InputSpec != "" {
			inputSpec = cfg.InputSpec
		}
		if outputDir == "" && cfg.OutputDir != "" {
			outputDir = cfg.OutputDir
		}
		if generatorName == "" && cfg.GeneratorName != "" {
			generatorName = cfg.GeneratorName
		}
		if templateDir == "" && cfg.TemplateDir != "" {
			templateDir = cfg.TemplateDir
		}
		if cfg.SkipValidation {
			skipValidation = true
		}
		if cfg.Verbose {
			verbose = true
		}
		// Merge additional properties from config (CLI takes precedence).
		// Iterate keys in sorted order so the merged slice is stable across runs.
		if cfg.AdditionalProperties != nil {
			cfgKeys := make([]string, 0, len(cfg.AdditionalProperties))
			for k := range cfg.AdditionalProperties {
				cfgKeys = append(cfgKeys, k)
			}
			sort.Strings(cfgKeys)
			for _, k := range cfgKeys {
				// Only add if not already specified via CLI
				found := false
				for _, prop := range additionalProperties {
					if strings.HasPrefix(prop, k+"=") {
						found = true
						break
					}
				}
				if !found {
					additionalProperties = append(additionalProperties, k+"="+cfg.AdditionalProperties[k])
				}
			}
		}
	}

	return gen.Generate(gen.Options{
		InputSpec:            inputSpec,
		OutputDir:            outputDir,
		GeneratorName:        generatorName,
		TemplateDir:          templateDir,
		AdditionalProperties: additionalProperties,
		SkipValidation:       skipValidation,
		Verbose:              verbose,
		Version:              version,
	})
}
