package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xseman/openapi-generator/internal/generator"
	"github.com/xseman/openapi-generator/internal/generator/dart"
	"github.com/xseman/openapi-generator/internal/generator/typescript"
	"github.com/xseman/openapi-generator/internal/template"
	"github.com/xseman/openapi-generator/templates"
)

// newTemplateEngine resolves and loads the Mustache templates for
// generatorName: an explicit templateDir takes priority, otherwise the
// well-known filesystem locations are searched, falling back to the
// templates embedded in the binary.
func newTemplateEngine(generatorName, templateDir string, verbose bool) (*template.Engine, error) {
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
			return nil, fmt.Errorf("failed to load template partials: %w", err)
		}
	} else {
		// Fall back to embedded templates
		if verbose {
			fmt.Println("Using embedded templates")
		}
		engine = template.NewEngineFromFS(templates.FS, generatorName)
		engine.Verbose = verbose
		if err := engine.LoadPartialsFromFS(); err != nil {
			return nil, fmt.Errorf("failed to load embedded template partials: %w", err)
		}
	}

	engine.RegisterDefaultLambdas()
	return engine, nil
}

// findTemplateDir searches well-known locations for a filesystem template
// directory for generatorName, returning "" if none exist (callers then
// fall back to the embedded templates).
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

// buildBaseData assembles the template data shared by every supporting,
// model, and API file: spec metadata, package paths, and the generator's
// additional properties.
func buildBaseData(spec *specData, apiPackage, modelPackage, version string, cg generator.CodegenConfig) map[string]any {
	baseData := map[string]any{
		"appName":          spec.info["title"],
		"appDescription":   spec.info["description"],
		"version":          spec.info["version"],
		"infoEmail":        spec.info["infoEmail"],
		"infoUrl":          spec.info["infoUrl"],
		"licenseName":      spec.info["licenseName"],
		"licenseUrl":       spec.info["licenseUrl"],
		"basePath":         spec.basePath,
		"host":             extractHost(spec.basePath),
		"generatorClass":   "TypeScriptFetchClientCodegen",
		"generatorVersion": version,
		"generatedDate":    time.Now().Format(time.RFC3339),
		"apiPackage":       apiPackage,
		"modelPackage":     modelPackage,
	}

	for k, v := range cg.GetAdditionalProperties() {
		baseData[k] = v
	}

	return baseData
}

// renderer carries the shared context needed to render supporting, model,
// and API files, and the per-folder index files, for one generation run.
type renderer struct {
	cg           generator.CodegenConfig
	tsGen        *typescript.FetchGenerator
	dartGen      *dart.FetchGenerator
	engine       *template.Engine
	outputDir    string
	apiPackage   string
	modelPackage string
	verbose      bool
}

// renderSupportingFiles renders every supporting file the generator
// declares (runtime, README, package manifests, ...), returning the
// output-relative paths written.
func (r *renderer) renderSupportingFiles(baseData map[string]any, spec *specData, modelMaps []map[string]any) ([]string, error) {
	if r.verbose {
		fmt.Println("Generating supporting files...")
	}

	var generatedFiles []string
	for _, sf := range r.cg.GetSupportingFiles() {
		data := copyMap(baseData)
		data["models"] = modelMaps
		data["hasModels"] = len(spec.models) > 0
		data["hasApis"] = len(spec.operationsByTag) > 0
		data["authMethods"] = template.ConvertSliceToMaps(spec.securitySchemes)

		outputPath := filepath.Join(r.outputDir, sf.Folder, sf.DestinationFilename)
		if r.verbose {
			fmt.Printf("  %s\n", outputPath)
		}

		if err := r.engine.RenderToFile(sf.TemplateFile, data, outputPath); err != nil {
			return nil, fmt.Errorf("failed to generate %s: %w", sf.DestinationFilename, err)
		}

		relPath := filepath.Join(sf.Folder, sf.DestinationFilename)
		if relPath != "" {
			generatedFiles = append(generatedFiles, relPath)
		}
	}

	return generatedFiles, nil
}

// renderModels renders every model template for every model, returning the
// output-relative paths written.
func (r *renderer) renderModels(baseData map[string]any, models []*generator.CodegenModel, modelMaps []map[string]any) ([]string, error) {
	if r.verbose {
		fmt.Println("Generating models...")
	}

	var generatedFiles []string
	modelTemplates := r.cg.GetModelTemplateFiles()
	for i, model := range models {
		for tmplFile, ext := range modelTemplates {
			data := copyMap(baseData)
			modelMap := modelMaps[i]
			data["model"] = modelMap
			data["models"] = []map[string]any{{"model": modelMap}}
			data["classname"] = model.Classname
			// hasImports should be true if we have regular imports OR oneOf imports
			data["hasImports"] = len(model.Imports) > 0 || len(model.OneOfModels) > 0 || len(model.OneOfArrays) > 0
			// A member model can appear both directly and inside an array member;
			// merge the two lists so each model is imported exactly once.
			oneOfImportNames := make([]string, 0, len(model.OneOfModels)+len(model.OneOfArrays))
			seenOneOf := make(map[string]bool)
			for _, n := range append(append([]string{}, model.OneOfModels...), model.OneOfArrays...) {
				if !seenOneOf[n] {
					seenOneOf[n] = true
					oneOfImportNames = append(oneOfImportNames, n)
				}
			}
			sort.Strings(oneOfImportNames)
			// The mustache engine has no -first/-last support, so the templates
			// gate one-time open/close blocks on these explicit flags instead.
			data["hasOneOfModels"] = len(model.OneOfModels) > 0
			data["hasOneOfArrays"] = len(model.OneOfArrays) > 0
			if r.tsGen != nil {
				data["tsImports"] = toTsImports(model.Imports, r.tsGen)
				data["oneOfImports"] = toTsImports(oneOfImportNames, r.tsGen)
			} else if r.dartGen != nil {
				data["dartImports"] = toDartImports(model.Imports, r.dartGen)
				data["oneOfImports"] = toDartImports(oneOfImportNames, r.dartGen)
			}
			// Skip importing Blob helpers if this model IS Blob (to avoid conflicts
			// with its own generated BlobFromJSON/BlobToJSON exports).
			isNotBlobModel := model.Classname != "Blob"
			data["isNotBlobModel"] = isNotBlobModel
			// True when at least one var's FromJSON/ToJSON body will literally call
			// the runtime's anyFromJSON/anyToJSON or BlobFromJSON/BlobToJSON helpers
			// (datatype resolved to "any"/"Blob" outside the
			// isFreeFormObject/isPrimitiveType/isArray/isMap fast paths that
			// modelGeneric.mustache passes through directly). Computed here so the
			// template can import those runtime helpers only when actually used,
			// instead of unconditionally importing them into every model file.
			if r.tsGen != nil {
				data["usesAnyJSON"] = modelUsesRuntimeJSONHelper(model, "any")
				data["usesBlobJSON"] = isNotBlobModel && modelUsesRuntimeJSONHelper(model, "Blob")
			}
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
						// Don't convert primitive types or parameterized/composite
						// declarations (Array<Foo>, A | B) - use them as-is
						if isPrimitiveTypeTS(itemStr) || strings.ContainsAny(itemStr, "<|& ") {
							parts = append(parts, itemStr)
						} else {
							typeName := r.cg.ToModelName(itemStr)
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

			outputPath := filepath.Join(r.outputDir, r.modelPackage, r.cg.ToModelFilename(model.Classname)+ext)
			if r.verbose {
				fmt.Printf("  %s\n", outputPath)
			}

			if err := r.engine.RenderToFile(tmplFile, data, outputPath); err != nil {
				return nil, fmt.Errorf("failed to generate model %s: %w", model.Classname, err)
			}

			relPath := filepath.Join(r.modelPackage, r.cg.ToModelFilename(model.Classname)+ext)
			generatedFiles = append(generatedFiles, relPath)
		}
	}

	return generatedFiles, nil
}

// renderAPIs renders every API template for every tag, returning the
// output-relative paths written.
func (r *renderer) renderAPIs(baseData map[string]any, operationsByTag map[string][]*generator.CodegenOperation) ([]string, error) {
	if r.verbose {
		fmt.Println("Generating APIs...")
	}

	var generatedFiles []string
	apiTemplates := r.cg.GetApiTemplateFiles()
	for _, tag := range sortedTags(operationsByTag) {
		ops := operationsByTag[tag]
		apiClassname := r.cg.ToApiName(tag)

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

			imports := collectApiImports(ops, r.cg)
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

			outputPath := filepath.Join(r.outputDir, r.apiPackage, r.cg.ToApiFilename(apiClassname)+ext)
			if r.verbose {
				fmt.Printf("  %s\n", outputPath)
			}

			if err := r.engine.RenderToFile(tmplFile, data, outputPath); err != nil {
				return nil, fmt.Errorf("failed to generate API %s: %w", apiClassname, err)
			}

			relPath := filepath.Join(r.apiPackage, r.cg.ToApiFilename(apiClassname)+ext)
			generatedFiles = append(generatedFiles, relPath)
		}
	}

	return generatedFiles, nil
}
