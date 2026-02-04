// Package main provides the CLI for the OpenAPI Generator Go implementation.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xseman/openapi-generator/internal/config"
	"github.com/xseman/openapi-generator/internal/generator"
	"github.com/xseman/openapi-generator/internal/generator/typescript"
	"github.com/xseman/openapi-generator/internal/parser"
	"github.com/xseman/openapi-generator/internal/template"
	"github.com/xseman/openapi-generator/templates"
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
			printTypeScriptFetchConfigHelp()
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
		// Merge additional properties from config (CLI takes precedence)
		if cfg.AdditionalProperties != nil {
			for k, v := range cfg.AdditionalProperties {
				// Only add if not already specified via CLI
				found := false
				for _, prop := range additionalProperties {
					if strings.HasPrefix(prop, k+"=") {
						found = true
						break
					}
				}
				if !found {
					additionalProperties = append(additionalProperties, k+"="+v)
				}
			}
		}
	}

	if verbose {
		fmt.Printf("Input spec: %s\n", inputSpec)
		fmt.Printf("Output dir: %s\n", outputDir)
		fmt.Printf("Generator: %s\n", generatorName)
	}

	// Validate required fields (after config file loading)
	if inputSpec == "" {
		return fmt.Errorf("input-spec is required (use -i flag or inputSpec in config file)")
	}
	if outputDir == "" {
		return fmt.Errorf("output is required (use -o flag or outputDir in config file)")
	}
	if generatorName == "" {
		return fmt.Errorf("generator-name is required (use -g flag or generatorName in config file)")
	}

	// Validate generator
	if generatorName != "typescript-fetch" {
		return fmt.Errorf("unsupported generator: %s (only 'typescript-fetch' is supported)", generatorName)
	}

	// Parse additional properties
	additionalProps := parseAdditionalProperties(additionalProperties)

	// Create generator configuration
	cfg := &config.GeneratorConfig{
		InputSpec:            inputSpec,
		OutputDir:            outputDir,
		GeneratorName:        generatorName,
		TemplateDir:          templateDir,
		AdditionalProperties: additionalProps,
	}

	// Create TypeScript config from additional properties
	tsConfig := config.NewTypeScriptFetchConfig()
	applyAdditionalProperties(tsConfig, additionalProps)

	// Create generator
	gen := typescript.NewFetchGenerator()
	gen.SetConfig(cfg)
	gen.TSConfig = tsConfig

	// Process options
	if err := gen.ProcessOpts(); err != nil {
		return fmt.Errorf("failed to process options: %w", err)
	}

	// Parse OpenAPI spec
	if verbose {
		fmt.Printf("Parsing OpenAPI spec: %s\n", inputSpec)
	}

	p := parser.NewParser()

	// Set up type conversion functions
	p.GetTypeFunc = gen.GetSchemaType
	p.ToModelNameFunc = gen.ToModelName
	p.ToVarNameFunc = gen.ToVarName

	// Set validation flag
	p.SkipValidation = skipValidation

	// Load spec
	if strings.HasPrefix(inputSpec, "http://") || strings.HasPrefix(inputSpec, "https://") {
		if err := p.LoadFromURL(inputSpec); err != nil {
			return fmt.Errorf("failed to load spec from URL: %w", err)
		}
	} else {
		if err := p.LoadFromFile(inputSpec); err != nil {
			return fmt.Errorf("failed to load spec from file: %w", err)
		}
	}

	// Get models and operations
	models, err := p.GetModels()
	if err != nil {
		return fmt.Errorf("failed to get models: %w", err)
	}

	operationsByTag, err := p.GetOperations()
	if err != nil {
		return fmt.Errorf("failed to get operations: %w", err)
	}

	// Detect and resolve operation ID conflicts within each tag
	for tag, ops := range operationsByTag {
		operationIDs := make(map[string]int)
		for i := range ops {
			opID := ops[i].OperationId
			if count, exists := operationIDs[opID]; exists {
				// Conflict detected - rename by appending suffix
				suffix := count + 1
				newID := fmt.Sprintf("%s%d", opID, suffix)
				if verbose {
					fmt.Printf("Warning: Duplicate operation ID '%s' in tag '%s', renaming to '%s'\n", opID, tag, newID)
				}
				ops[i].OperationId = newID
				ops[i].Nickname = newID
				operationIDs[opID] = suffix
			} else {
				operationIDs[opID] = 0
			}
		}
	}

	securitySchemes, err := p.GetSecuritySchemes()
	if err != nil {
		return fmt.Errorf("failed to get security schemes: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d models\n", len(models))
		opCount := 0
		for _, ops := range operationsByTag {
			opCount += len(ops)
		}
		fmt.Printf("Found %d operations in %d tags\n", opCount, len(operationsByTag))
	}

	// Post-process models
	models = gen.PostProcessModels(models)

	// Post-process operations
	for tag, ops := range operationsByTag {
		operationsByTag[tag] = gen.PostProcessOperations(ops)
	}

	// Set up template engine
	tmplDir := templateDir
	if tmplDir == "" {
		tmplDir = findTemplateDir(generatorName)
	}

	var engine *template.Engine

	if tmplDir != "" {
		// Use filesystem templates
		if verbose {
			fmt.Printf("Using templates from: %s\n", tmplDir)
		}
		engine = template.NewEngine(tmplDir)
		engine.Verbose = verbose
		if err := engine.LoadPartials(); err != nil {
			return fmt.Errorf("failed to load template partials: %w", err)
		}
	} else {
		// Fall back to embedded templates
		if verbose {
			fmt.Println("Using embedded templates")
		}
		engine = template.NewEngineFromFS(templates.FS, generatorName)
		engine.Verbose = verbose
		if err := engine.LoadPartialsFromFS(); err != nil {
			return fmt.Errorf("failed to load embedded template partials: %w", err)
		}
	}

	engine.RegisterDefaultLambdas()

	// Prepare template data
	info := p.GetInfo()
	basePath := p.GetBasePath()

	baseData := map[string]any{
		"appName":          info["title"],
		"appDescription":   info["description"],
		"version":          info["version"],
		"infoEmail":        info["infoEmail"],
		"infoUrl":          info["infoUrl"],
		"licenseName":      info["licenseName"],
		"licenseUrl":       info["licenseUrl"],
		"basePath":         basePath,
		"host":             extractHost(basePath),
		"generatorClass":   "TypeScriptFetchClientCodegen",
		"generatorVersion": version,
		"generatedDate":    time.Now().Format(time.RFC3339),
		"apiPackage":       gen.ApiPackage,
		"modelPackage":     gen.ModelPackage,
	}

	// Merge additional properties
	for k, v := range gen.GetAdditionalProperties() {
		baseData[k] = v
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Track generated files for metadata
	var generatedFiles []string

	// Generate supporting files
	if verbose {
		fmt.Println("Generating supporting files...")
	}

	// Convert models to maps for template rendering and preprocess for Mustache compatibility
	modelMaps := template.ConvertSliceToMaps(models)
	modelMaps = template.PreprocessModelData(modelMaps)

	for _, sf := range gen.GetSupportingFiles() {
		data := copyMap(baseData)
		data["models"] = modelMaps
		data["hasModels"] = len(models) > 0
		data["hasApis"] = len(operationsByTag) > 0
		data["authMethods"] = template.ConvertSliceToMaps(securitySchemes)

		outputPath := filepath.Join(outputDir, sf.Folder, sf.DestinationFilename)
		if verbose {
			fmt.Printf("  %s\n", outputPath)
		}

		if err := engine.RenderToFile(sf.TemplateFile, data, outputPath); err != nil {
			return fmt.Errorf("failed to generate %s: %w", sf.DestinationFilename, err)
		}

		// Track generated file (relative to output dir)
		relPath := filepath.Join(sf.Folder, sf.DestinationFilename)
		if relPath != "" {
			generatedFiles = append(generatedFiles, relPath)
		}
	}

	// Generate models
	if verbose {
		fmt.Println("Generating models...")
	}

	modelTemplates := gen.GetModelTemplateFiles()
	for i, model := range models {
		for tmplFile, ext := range modelTemplates {
			data := copyMap(baseData)
			modelMap := modelMaps[i]
			data["model"] = modelMap
			data["models"] = []map[string]any{{"model": modelMap}}
			data["classname"] = model.Classname
			// hasImports should be true if we have regular imports OR oneOf imports
			data["hasImports"] = len(model.Imports) > 0 || len(model.OneOfModels) > 0
			data["tsImports"] = toTsImports(model.Imports, gen)
			// Add oneOfImports with proper filename conversion (separate from oneOfModels string array)
			data["oneOfImports"] = toTsImports(model.OneOfModels, gen)
			// Skip importing Blob helpers if this model IS Blob (to avoid conflicts)
			data["isNotBlobModel"] = model.Classname != "Blob"
			// Add model-level properties at top level for template access
			for k, v := range modelMap {
				if _, exists := data[k]; !exists {
					data[k] = v
				}
			}

			// For oneOf, create a joined string since mustache doesn't support -last
			if oneOf, ok := modelMap["oneOf"]; ok {
				if oneOfArray, isArray := oneOf.([]any); isArray && len(oneOfArray) > 0 {
					parts := make([]string, 0, len(oneOfArray))
					for _, item := range oneOfArray {
						itemStr := fmt.Sprintf("%v", item)
						// Don't convert primitive types - use them as-is
						if isPrimitiveTypeTS(itemStr) {
							parts = append(parts, itemStr)
						} else {
							typeName := gen.ToModelName(itemStr)
							if typeName != "" {
								parts = append(parts, typeName)
							}
						}
					}
					if len(parts) > 0 {
						data["oneOfJoined"] = strings.Join(parts, " | ")
					} else {
						data["oneOfJoined"] = "any"
					}
				}
			}

			outputPath := filepath.Join(outputDir, gen.ModelPackage, gen.ToModelFilename(model.Classname)+ext)
			if verbose {
				fmt.Printf("  %s\n", outputPath)
			}

			if err := engine.RenderToFile(tmplFile, data, outputPath); err != nil {
				return fmt.Errorf("failed to generate model %s: %w", model.Classname, err)
			}

			// Track generated file (relative to output dir)
			relPath := filepath.Join(gen.ModelPackage, gen.ToModelFilename(model.Classname)+ext)
			generatedFiles = append(generatedFiles, relPath)
		}
	}

	// Generate APIs
	if verbose {
		fmt.Println("Generating APIs...")
	}

	apiTemplates := gen.GetApiTemplateFiles()
	for tag, ops := range operationsByTag {
		apiClassname := gen.ToApiName(tag)

		// Convert operations to maps and preprocess for Mustache compatibility
		opMaps := template.ConvertSliceToMaps(ops)
		opMaps = template.PreprocessOperationData(opMaps)

		for tmplFile, ext := range apiTemplates {
			data := copyMap(baseData)
			data["classname"] = apiClassname
			data["classVarName"] = strings.ToLower(apiClassname[:1]) + apiClassname[1:]
			data["operations"] = map[string]any{
				"operation": opMaps,
				"classname": apiClassname,
			}
			data["operation"] = opMaps

			imports := collectApiImports(ops, gen)
			data["imports"] = imports
			data["hasImports"] = len(imports) > 0

			// Check if any operation has enum parameters
			hasEnums := false
			for _, op := range ops {
				for _, param := range op.AllParams {
					if param.IsEnum {
						hasEnums = true
						break
					}
				}
				if hasEnums {
					break
				}
			}
			data["hasEnums"] = hasEnums

			outputPath := filepath.Join(outputDir, gen.ApiPackage, gen.ToApiFilename(apiClassname)+ext)
			if verbose {
				fmt.Printf("  %s\n", outputPath)
			}

			if err := engine.RenderToFile(tmplFile, data, outputPath); err != nil {
				return fmt.Errorf("failed to generate API %s: %w", apiClassname, err)
			}

			// Track generated file (relative to output dir)
			relPath := filepath.Join(gen.ApiPackage, gen.ToApiFilename(apiClassname)+ext)
			generatedFiles = append(generatedFiles, relPath)
		}
	}

	// Generate index files
	if len(models) > 0 {
		modelIndex := generateModelIndex(models, gen)
		modelIndexPath := filepath.Join(outputDir, gen.ModelPackage, "index.ts")
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(modelIndexPath), 0755); err != nil {
			return fmt.Errorf("failed to create model index directory: %w", err)
		}
		if err := os.WriteFile(modelIndexPath, []byte(modelIndex), 0600); err != nil {
			return fmt.Errorf("failed to write model index: %w", err)
		}
		generatedFiles = append(generatedFiles, filepath.Join(gen.ModelPackage, "index.ts"))
	}

	if len(operationsByTag) > 0 {
		apiIndex := generateApiIndex(operationsByTag, gen)
		apiIndexPath := filepath.Join(outputDir, gen.ApiPackage, "index.ts")
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(apiIndexPath), 0755); err != nil {
			return fmt.Errorf("failed to create API index directory: %w", err)
		}
		if err := os.WriteFile(apiIndexPath, []byte(apiIndex), 0600); err != nil {
			return fmt.Errorf("failed to write API index: %w", err)
		}
		generatedFiles = append(generatedFiles, filepath.Join(gen.ApiPackage, "index.ts"))
	}

	// Generate .openapi-generator metadata
	if err := generateMetadata(outputDir, generatedFiles, version); err != nil {
		return fmt.Errorf("failed to generate metadata: %w", err)
	}

	fmt.Printf("\nGeneration complete! Output written to: %s\n", outputDir)
	return nil
}

func parseAdditionalProperties(props []string) map[string]any {
	result := make(map[string]any)
	for _, prop := range props {
		parts := strings.SplitN(prop, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch strings.ToLower(value) {
			case "true":
				result[key] = true
			case "false":
				result[key] = false
			default:
				result[key] = value
			}
		}
	}
	return result
}

func applyAdditionalProperties(tsConfig *config.TypeScriptFetchConfig, props map[string]any) {
	if v, ok := props["withPackageJson"].(bool); ok {
		tsConfig.WithPackageJson = v
	}
	if v, ok := props["withInterfaces"].(bool); ok {
		tsConfig.WithInterfaces = v
	}
	if v, ok := props["useSingleRequestParameter"].(bool); ok {
		tsConfig.UseSingleRequestParameter = v
	}
	if v, ok := props["prefixParameterInterfaces"].(bool); ok {
		tsConfig.PrefixParameterInterfaces = v
	}
	if v, ok := props["withoutRuntimeChecks"].(bool); ok {
		tsConfig.WithoutRuntimeChecks = v
	}
	if v, ok := props["stringEnums"].(bool); ok {
		tsConfig.StringEnums = v
	}
	if v, ok := props["importFileExtension"].(string); ok {
		tsConfig.ImportFileExtension = v
	}
	if v, ok := props["fileNaming"].(string); ok {
		tsConfig.FileNaming = v
	}
	if v, ok := props["validationAttributes"].(bool); ok {
		tsConfig.GenerateValidationAttributes = v
	}
}

func findTemplateDir(generatorName string) string {
	locations := []string{
		filepath.Join(".", "templates", generatorName),
		filepath.Join(".", generatorName),
		filepath.Join(os.Getenv("HOME"), ".openapi-generator", "templates", generatorName),
		filepath.Join("/usr/share/openapi-generator/templates", generatorName),
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		locations = append(locations,
			filepath.Join(exeDir, "templates", generatorName),
			filepath.Join(exeDir, "..", "templates", generatorName),
		)
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

func extractHost(basePath string) string {
	if strings.HasPrefix(basePath, "http://") || strings.HasPrefix(basePath, "https://") {
		parts := strings.SplitN(basePath, "/", 4)
		if len(parts) >= 3 {
			return parts[2]
		}
	}
	return ""
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}

func toTsImports(imports []string, gen *typescript.FetchGenerator) []map[string]string {
	result := make([]map[string]string, 0, len(imports))
	for _, imp := range imports {
		className := gen.ToModelName(imp)
		// Skip empty class names and primitive types
		if className == "" || gen.IsPrimitive(className) {
			continue
		}
		result = append(result, map[string]string{
			"classname": className,
			"filename":  gen.ToModelFilename(className),
		})
	}
	return result
}

// isPrimitiveTypeTS checks if a type string is a TypeScript primitive type.
// Returns true for built-in types like string, number, boolean, etc.
func isPrimitiveTypeTS(t string) bool {
	primitives := map[string]bool{
		"string": true, "number": true, "boolean": true,
		"any": true, "void": true, "null": true,
		"Date": true, "Blob": true, "undefined": true,
	}
	return primitives[t]
}

func collectApiImports(ops []*generator.CodegenOperation, gen generator.CodegenConfig) []map[string]string {
	imports := make(map[string]bool)
	for _, op := range ops {
		for _, imp := range op.Imports {
			imports[imp] = true
		}
	}

	// Get primitives map to filter
	primitives := gen.GetLanguageSpecificPrimitives()

	result := make([]map[string]string, 0, len(imports))
	for imp := range imports {
		// Convert the import name to the proper model class name
		className := gen.ToModelName(imp)
		// Skip empty class names and primitive types
		if className == "" || primitives[className] {
			continue
		}
		result = append(result, map[string]string{
			"import":    imp,
			"classname": className,
			"className": className, // Template expects className (camelCase)
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i]["classname"] < result[j]["classname"]
	})

	return result
}

func generateModelIndex(models []*generator.CodegenModel, gen *typescript.FetchGenerator) string {
	var sb strings.Builder
	sb.WriteString("/* tslint:disable */\n")
	sb.WriteString("/* eslint-disable */\n")
	sb.WriteString("\n")

	// Export helper functions from runtime
	sb.WriteString("// Re-export helper functions from runtime\n")
	sb.WriteString("export {\n")
	sb.WriteString("    anyFromJSON,\n")
	sb.WriteString("    anyToJSON,\n")
	sb.WriteString("    stringFromJSON,\n")
	sb.WriteString("    stringToJSON,\n")
	sb.WriteString("    DateFromJSON,\n")
	sb.WriteString("    BlobFromJSON,\n")
	sb.WriteString("    BlobToJSON,\n")
	sb.WriteString("    FromJSON,\n")
	sb.WriteString("} from '../runtime';\n")
	sb.WriteString("\n")

	// Add ModelObject type for generic object schemas
	sb.WriteString("// Generic object type for unstructured schemas\n")
	sb.WriteString("export type ModelObject = Record<string, any>;\n")
	sb.WriteString("export function ModelObjectFromJSON(json: any): ModelObject {\n")
	sb.WriteString("    return json;\n")
	sb.WriteString("}\n")
	sb.WriteString("export function ModelObjectToJSON(value: ModelObject): any {\n")
	sb.WriteString("    return value;\n")
	sb.WriteString("}\n")
	sb.WriteString("\n")

	for _, model := range models {
		filename := gen.ToModelFilename(model.Classname)
		ext := gen.ImportFileExtension
		sb.WriteString(fmt.Sprintf("export * from './%s%s';\n", filename, ext))
	}

	return sb.String()
}

func generateApiIndex(ops map[string][]*generator.CodegenOperation, gen *typescript.FetchGenerator) string {
	var sb strings.Builder
	sb.WriteString("/* tslint:disable */\n")
	sb.WriteString("/* eslint-disable */\n")

	for tag := range ops {
		apiClassname := gen.ToApiName(tag)
		filename := gen.ToApiFilename(apiClassname)
		ext := gen.ImportFileExtension
		sb.WriteString(fmt.Sprintf("export * from './%s%s';\n", filename, ext))
	}

	return sb.String()
}

func printTypeScriptFetchConfigHelp() {
	fmt.Println("CONFIG OPTIONS for typescript-fetch:")
	fmt.Println()
	fmt.Println("  withPackageJson")
	fmt.Println("      Generate package.json and tsconfig.json files. (Default: false)")
	fmt.Println()
	fmt.Println("  withInterfaces")
	fmt.Println("      Generate interfaces alongside classes. (Default: false)")
	fmt.Println()
	fmt.Println("  useSingleRequestParameter")
	fmt.Println("      Use single request object for method parameters. (Default: true)")
	fmt.Println()
	fmt.Println("  prefixParameterInterfaces")
	fmt.Println("      Prefix parameter interfaces with API class name. (Default: false)")
	fmt.Println()
	fmt.Println("  withoutRuntimeChecks")
	fmt.Println("      Skip runtime type validation (FromJSON/ToJSON). (Default: false)")
	fmt.Println()
	fmt.Println("  stringEnums")
	fmt.Println("      Generate string enums instead of const objects. (Default: false)")
	fmt.Println()
	fmt.Println("  importFileExtension")
	fmt.Println("      File extension for imports (e.g., '.js' for ESM). (Default: '')")
	fmt.Println()
	fmt.Println("  fileNaming")
	fmt.Println("      File naming convention: PascalCase, camelCase, kebab-case.")
	fmt.Println("      (Default: kebab-case)")
	fmt.Println()
	fmt.Println("  validationAttributes")
	fmt.Println("      Generate validation metadata. (Default: false)")
	fmt.Println()
}

// generateMetadata creates the .openapi-generator folder with FILES and VERSION
func generateMetadata(outputDir string, generatedFiles []string, version string) error {
	metaDir := filepath.Join(outputDir, ".openapi-generator")

	// Create .openapi-generator directory
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return fmt.Errorf("failed to create .openapi-generator directory: %w", err)
	}

	// Sort files for consistent output
	sort.Strings(generatedFiles)

	// Generate FILES content
	var filesContent strings.Builder
	for _, file := range generatedFiles {
		// Normalize path separators to forward slashes
		normalizedPath := filepath.ToSlash(file)
		filesContent.WriteString(normalizedPath)
		filesContent.WriteString("\n")
	}

	// Write FILES
	filesPath := filepath.Join(metaDir, "FILES")
	if err := os.WriteFile(filesPath, []byte(filesContent.String()), 0600); err != nil {
		return fmt.Errorf("failed to write FILES: %w", err)
	}

	// Write VERSION
	versionPath := filepath.Join(metaDir, "VERSION")
	versionContent := fmt.Sprintf("%s\n", version)
	if err := os.WriteFile(versionPath, []byte(versionContent), 0600); err != nil {
		return fmt.Errorf("failed to write VERSION: %w", err)
	}

	return nil
}
