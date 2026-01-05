// Package template provides Mustache template rendering functionality.
package template

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cbroglie/mustache"
)

// Engine handles Mustache template rendering.
type Engine struct {
	// TemplateDir is the directory containing templates
	TemplateDir string

	// Partials cache
	partials map[string]string

	// Custom lambdas for templates
	Lambdas map[string]func(text string, render mustache.RenderFunc) (string, error)

	// Verbose enables debug logging of template execution
	Verbose bool
}

// NewEngine creates a new template engine.
func NewEngine(templateDir string) *Engine {
	return &Engine{
		TemplateDir: templateDir,
		partials:    make(map[string]string),
		Lambdas:     make(map[string]func(text string, render mustache.RenderFunc) (string, error)),
	}
}

// LoadPartials loads all partial templates from the template directory.
func (e *Engine) LoadPartials() error {
	return filepath.WalkDir(e.TemplateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".mustache") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read partial %s: %w", path, err)
		}

		// Get relative name without extension
		relPath, _ := filepath.Rel(e.TemplateDir, path)
		name := strings.TrimSuffix(relPath, ".mustache")
		name = strings.ReplaceAll(name, string(filepath.Separator), "/")

		e.partials[name] = string(content)
		if e.Verbose {
			fmt.Printf("[TEMPLATE] Loaded partial: %s\n", name)
		}
		return nil
	})
}

// RegisterLambda registers a custom lambda function.
func (e *Engine) RegisterLambda(name string, fn func(text string, render mustache.RenderFunc) (string, error)) {
	e.Lambdas[name] = fn
}

// RegisterDefaultLambdas registers the default set of lambdas used by openapi-generator.
func (e *Engine) RegisterDefaultLambdas() {
	// lowercase lambda
	e.RegisterLambda("lowercase", func(text string, render mustache.RenderFunc) (string, error) {
		rendered, err := render(text)
		if err != nil {
			return "", err
		}
		return strings.ToLower(rendered), nil
	})

	// uppercase lambda
	e.RegisterLambda("uppercase", func(text string, render mustache.RenderFunc) (string, error) {
		rendered, err := render(text)
		if err != nil {
			return "", err
		}
		return strings.ToUpper(rendered), nil
	})

	// camelcase lambda
	e.RegisterLambda("camelcase", func(text string, render mustache.RenderFunc) (string, error) {
		rendered, err := render(text)
		if err != nil {
			return "", err
		}
		return toCamelCase(rendered), nil
	})

	// pascalcase lambda
	e.RegisterLambda("pascalcase", func(text string, render mustache.RenderFunc) (string, error) {
		rendered, err := render(text)
		if err != nil {
			return "", err
		}
		return toPascalCase(rendered), nil
	})

	// snakecase lambda
	e.RegisterLambda("snakecase", func(text string, render mustache.RenderFunc) (string, error) {
		rendered, err := render(text)
		if err != nil {
			return "", err
		}
		return toSnakeCase(rendered), nil
	})

	// indented_star_1 - adds " * " prefix with 1 space indentation
	e.RegisterLambda("indented_star_1", func(text string, render mustache.RenderFunc) (string, error) {
		rendered, err := render(text)
		if err != nil {
			return "", err
		}
		return indentWithPrefix(rendered, 1, " ", "* "), nil
	})

	// indented_star_4 - adds " * " prefix with 4 space indentation
	e.RegisterLambda("indented_star_4", func(text string, render mustache.RenderFunc) (string, error) {
		rendered, err := render(text)
		if err != nil {
			return "", err
		}
		return indentWithPrefix(rendered, 5, " ", "* "), nil
	})

	// indented_1 - indents by 1 level (4 spaces)
	e.RegisterLambda("indented_1", func(text string, render mustache.RenderFunc) (string, error) {
		rendered, err := render(text)
		if err != nil {
			return "", err
		}
		return indent(rendered, "    "), nil
	})

	// indented_2 - indents by 2 levels (8 spaces)
	e.RegisterLambda("indented_2", func(text string, render mustache.RenderFunc) (string, error) {
		rendered, err := render(text)
		if err != nil {
			return "", err
		}
		return indent(rendered, "        "), nil
	})
}

// Render renders a template with the given data.
func (e *Engine) Render(templateName string, data any) (string, error) {
	if e.Verbose {
		fmt.Printf("[TEMPLATE] Rendering template: %s\n", templateName)
	}
	templatePath := filepath.Join(e.TemplateDir, templateName)

	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templateName, err)
	}

	return e.RenderString(string(content), data)
}

// RenderString renders a template string with the given data.
func (e *Engine) RenderString(template string, data any) (string, error) {
	// Create partial provider
	provider := &partialProvider{partials: e.partials, Verbose: e.Verbose}

	// Merge data with lambdas
	mergedData := e.mergeDataWithLambdas(data)

	// Parse and render
	tmpl, err := mustache.ParseStringPartials(template, provider)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	result, err := tmpl.Render(mergedData)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return result, nil
}

// RenderToFile renders a template and writes to a file.
func (e *Engine) RenderToFile(templateName string, data any, outputPath string) error {
	if e.Verbose {
		fmt.Printf("[TEMPLATE] Processing: %s -> %s\n", templateName, outputPath)
	}
	result, err := e.Render(templateName, data)
	if err != nil {
		return err
	}

	// Create output directory if needed
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", outputPath, err)
	}

	if e.Verbose {
		fmt.Printf("[TEMPLATE] Generated: %s\n", outputPath)
	}

	return nil
}

// mergeDataWithLambdas merges the data with lambda functions.
func (e *Engine) mergeDataWithLambdas(data any) any {
	// Convert data to map if possible
	dataMap, ok := data.(map[string]any)
	if !ok {
		// Try to use reflection to convert struct to map
		dataMap = structToMap(data)
		if dataMap == nil {
			return data
		}
	}

	// Add lambdas under "lambda" key
	lambdaMap := make(map[string]any)
	for name, fn := range e.Lambdas {
		lambdaMap[name] = fn
	}
	dataMap["lambda"] = lambdaMap

	return dataMap
}

// partialProvider implements mustache.PartialProvider
type partialProvider struct {
	partials map[string]string
	Verbose  bool
}

func (p *partialProvider) Get(name string) (string, error) {
	if partial, ok := p.partials[name]; ok {
		if p.Verbose {
			fmt.Printf("[TEMPLATE]   -> Using partial: %s\n", name)
		}
		return partial, nil
	}
	return "", fmt.Errorf("partial not found: %s", name)
}

// Helper functions

func toCamelCase(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return s
	}
	result := strings.ToLower(words[0])
	for _, word := range words[1:] {
		if len(word) > 0 {
			result += strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return result
}

func toPascalCase(s string) string {
	words := splitWords(s)
	result := ""
	for _, word := range words {
		if len(word) > 0 {
			result += strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return result
}

func toSnakeCase(s string) string {
	words := splitWords(s)
	result := ""
	for i, word := range words {
		if i > 0 {
			result += "_"
		}
		result += strings.ToLower(word)
	}
	return result
}

func splitWords(s string) []string {
	var words []string
	var current strings.Builder

	for i, r := range s {
		if !isAlphanumeric(r) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			continue
		}

		if i > 0 && isUpperCase(r) && current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isUpperCase(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func indent(s string, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

func indentWithPrefix(s string, spaces int, spacer, prefix string) string {
	lines := strings.Split(s, "\n")
	indentation := strings.Repeat(spacer, spaces)
	for i, line := range lines {
		if line != "" {
			lines[i] = indentation + prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// structToMap converts a struct to a map using JSON marshaling.
// This ensures all fields are properly accessible in Mustache templates.
func structToMap(data any) map[string]any {
	if data == nil {
		return nil
	}

	// If already a map, return as is
	if m, ok := data.(map[string]any); ok {
		return m
	}

	// Use JSON marshal/unmarshal for conversion
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil
	}

	return result
}

// ToTemplateData converts any struct or data to a map suitable for template rendering.
func ToTemplateData(data any) map[string]any {
	return structToMap(data)
}

// ConvertSliceToMaps converts a slice of structs to a slice of maps.
func ConvertSliceToMaps(slice any) []map[string]any {
	jsonBytes, err := json.Marshal(slice)
	if err != nil {
		return nil
	}

	var result []map[string]any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil
	}

	return result
}

// PreprocessOperationData adds synthetic fields for Mustache template compatibility.
// The Java Mustache library supports ".0" syntax for checking if an array has elements,
// but the Go cbroglie/mustache library does not. This function adds boolean "has*" fields.
func PreprocessOperationData(opMaps []map[string]any) []map[string]any {
	for _, op := range opMaps {
		addHasArrayFlag(op, "allParams", "hasAllParams")
		addHasArrayFlag(op, "queryParams", "hasQueryParams")
		addHasArrayFlag(op, "pathParams", "hasPathParams")
		addHasArrayFlag(op, "headerParams", "hasHeaderParams")
		addHasArrayFlag(op, "cookieParams", "hasCookieParams")
		addHasArrayFlag(op, "bodyParams", "hasBodyParams")
		addHasArrayFlag(op, "formParams", "hasFormParams")
		addHasArrayFlag(op, "requiredParams", "hasRequiredParams")
		addHasArrayFlag(op, "optionalParams", "hasOptionalParams")
		addHasArrayFlag(op, "responses", "hasResponses")
		addHasArrayFlag(op, "produces", "hasProduces")
		addHasArrayFlag(op, "consumes", "hasConsumes")
		addHasArrayFlag(op, "authMethods", "hasAuthMethods")
	}
	return opMaps
}

// PreprocessModelData adds synthetic fields for Mustache template compatibility.
func PreprocessModelData(modelMaps []map[string]any) []map[string]any {
	for _, model := range modelMaps {
		addHasArrayFlag(model, "vars", "hasVars")
		addHasArrayFlag(model, "requiredVars", "hasRequiredVars")
		addHasArrayFlag(model, "optionalVars", "hasOptionalVars")
		addHasArrayFlag(model, "allVars", "hasAllVars")
		addHasArrayFlag(model, "readWriteVars", "hasReadWriteVars")
		addHasArrayFlag(model, "parentVars", "hasParentVars")
		addHasArrayFlag(model, "imports", "hasImports")
	}
	return modelMaps
}

// addHasArrayFlag adds a boolean flag indicating if the array has elements.
func addHasArrayFlag(data map[string]any, arrayKey, flagKey string) {
	if arr, ok := data[arrayKey]; ok && arr != nil {
		switch v := arr.(type) {
		case []any:
			data[flagKey] = len(v) > 0
		case []map[string]any:
			data[flagKey] = len(v) > 0
		case []string:
			data[flagKey] = len(v) > 0
		default:
			data[flagKey] = false
		}
	} else {
		data[flagKey] = false
	}
}
