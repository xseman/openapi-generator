// Package gen implements the OpenAPI Generator generation pipeline: it
// parses an OpenAPI spec, assembles codegen models/operations, renders them
// through the generator's Mustache templates, and writes the resulting
// files (plus per-folder index files and .openapi-generator metadata) to
// disk.
//
// It is deliberately independent of the CLI: callers build an Options value
// and call Generate. cmd/openapi-generator wires cobra flags and a config
// file onto Options and reports the returned error.
package gen

import (
	"fmt"
	"os"

	"github.com/xseman/openapi-generator/internal/config"
	"github.com/xseman/openapi-generator/internal/generator"
	"github.com/xseman/openapi-generator/internal/generator/dart"
	"github.com/xseman/openapi-generator/internal/generator/typescript"
	"github.com/xseman/openapi-generator/internal/template"
)

// Options configures a single generation run.
type Options struct {
	// InputSpec is a local file path or an http(s) URL to the OpenAPI
	// document to generate from.
	InputSpec string
	// OutputDir is the directory generated files are written to.
	OutputDir string
	// GeneratorName selects the target generator: "typescript-fetch" or
	// "dart-fetch".
	GeneratorName string
	// TemplateDir, if set, overrides the embedded templates with templates
	// loaded from this filesystem directory.
	TemplateDir string
	// AdditionalProperties holds "key=value" generator options, as passed
	// via the CLI's --additional-properties flag.
	AdditionalProperties []string
	// SkipValidation disables OpenAPI spec validation.
	SkipValidation bool
	// Verbose enables progress logging to stdout.
	Verbose bool
	// Version is the openapi-generator version string embedded in
	// generated metadata (.openapi-generator/VERSION, generatorVersion).
	Version string
}

// Generate runs the full pipeline for opts: parse the spec, assemble
// models/operations, render templates, and write the generated files,
// index files, and .openapi-generator metadata to opts.OutputDir.
func Generate(opts Options) error {
	if opts.Verbose {
		fmt.Printf("Input spec: %s\n", opts.InputSpec)
		fmt.Printf("Output dir: %s\n", opts.OutputDir)
		fmt.Printf("Generator: %s\n", opts.GeneratorName)
	}

	if opts.InputSpec == "" {
		return fmt.Errorf("input-spec is required (use -i flag or inputSpec in config file)")
	}
	if opts.OutputDir == "" {
		return fmt.Errorf("output is required (use -o flag or outputDir in config file)")
	}
	if opts.GeneratorName == "" {
		return fmt.Errorf("generator-name is required (use -g flag or generatorName in config file)")
	}
	if opts.GeneratorName != "typescript-fetch" && opts.GeneratorName != "dart-fetch" {
		return fmt.Errorf("unsupported generator: %s (supported: 'typescript-fetch', 'dart-fetch')", opts.GeneratorName)
	}

	additionalProps := parseAdditionalProperties(opts.AdditionalProperties)

	cfg := &config.GeneratorConfig{
		InputSpec:            opts.InputSpec,
		OutputDir:            opts.OutputDir,
		GeneratorName:        opts.GeneratorName,
		TemplateDir:          opts.TemplateDir,
		AdditionalProperties: additionalProps,
	}

	// Construct the generator (TypeScript or Dart) and keep both the
	// abstract CodegenConfig (for parser/template plumbing) and the
	// concrete generator (for language-specific helpers like index-file
	// generation).
	var (
		cg      generator.CodegenConfig
		tsGen   *typescript.FetchGenerator
		dartGen *dart.FetchGenerator
	)
	switch opts.GeneratorName {
	case "typescript-fetch":
		tsConfig := config.NewTypeScriptFetchConfig()
		applyTypeScriptAdditionalProperties(tsConfig, additionalProps)
		tsGen = typescript.NewFetchGenerator()
		tsGen.SetConfig(cfg)
		tsGen.TSConfig = tsConfig
		cg = tsGen
	case "dart-fetch":
		dartConfig := config.NewDartFetchConfig()
		applyDartAdditionalProperties(dartConfig, additionalProps)
		dartGen = dart.NewFetchGenerator()
		dartGen.SetConfig(cfg)
		dartGen.DartConfig = dartConfig
		cg = dartGen
	}

	if err := cg.ProcessOpts(); err != nil {
		return fmt.Errorf("failed to process options: %w", err)
	}

	// Capture per-language paths/extensions used by the generation pipeline.
	var apiPackage, modelPackage string
	switch {
	case tsGen != nil:
		apiPackage = tsGen.ApiPackage
		modelPackage = tsGen.ModelPackage
	case dartGen != nil:
		apiPackage = dartGen.ApiPackage
		modelPackage = dartGen.ModelPackage
	}

	spec, err := assembleSpec(cg, opts)
	if err != nil {
		return err
	}

	engine, err := newTemplateEngine(opts.GeneratorName, opts.TemplateDir, opts.Verbose)
	if err != nil {
		return err
	}

	baseData := buildBaseData(spec, apiPackage, modelPackage, opts.Version, cg)

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	resolveDiscriminatorFilenames(spec.models, cg)

	// Convert models to maps for template rendering and preprocess for
	// Mustache compatibility.
	modelMaps := template.ConvertSliceToMaps(spec.models)
	modelMaps = template.PreprocessModelData(modelMaps)

	r := &renderer{
		cg:           cg,
		tsGen:        tsGen,
		dartGen:      dartGen,
		engine:       engine,
		outputDir:    opts.OutputDir,
		apiPackage:   apiPackage,
		modelPackage: modelPackage,
		verbose:      opts.Verbose,
	}

	var generatedFiles []string

	supportingFiles, err := r.renderSupportingFiles(baseData, spec, modelMaps)
	if err != nil {
		return err
	}
	generatedFiles = append(generatedFiles, supportingFiles...)

	modelFiles, err := r.renderModels(baseData, spec.models, modelMaps)
	if err != nil {
		return err
	}
	generatedFiles = append(generatedFiles, modelFiles...)

	apiFiles, err := r.renderAPIs(baseData, spec.operationsByTag)
	if err != nil {
		return err
	}
	generatedFiles = append(generatedFiles, apiFiles...)

	indexFiles, err := r.writeIndexFiles(spec.models, spec.operationsByTag)
	if err != nil {
		return err
	}
	generatedFiles = append(generatedFiles, indexFiles...)

	if err := generateMetadata(opts.OutputDir, generatedFiles, opts.Version); err != nil {
		return fmt.Errorf("failed to generate metadata: %w", err)
	}

	fmt.Printf("\nGeneration complete! Output written to: %s\n", opts.OutputDir)
	return nil
}
