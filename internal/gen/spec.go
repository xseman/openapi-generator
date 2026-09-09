package gen

import (
	"fmt"
	"os"
	"strings"

	"github.com/xseman/openapi-generator/internal/generator"
	"github.com/xseman/openapi-generator/internal/parser"
)

// specData is the assembled result of turning an OpenAPI document into
// codegen models, operations, and security schemes, ready for template
// rendering.
type specData struct {
	models          []*generator.CodegenModel
	operationsByTag map[string][]*generator.CodegenOperation
	securitySchemes []*generator.CodegenSecurity
	info            map[string]string
	basePath        string
}

// assembleSpec parses opts.InputSpec, converts it into codegen models,
// operations and security schemes via cg, resolves duplicate operation IDs,
// and runs the generator's post-processing hooks.
func assembleSpec(cg generator.CodegenConfig, opts Options) (*specData, error) {
	p := parser.NewParser()

	// Set up type conversion functions
	p.GetTypeFunc = cg.GetSchemaType
	p.ToModelNameFunc = cg.ToModelName
	p.ToVarNameFunc = cg.ToVarName

	p.SkipValidation = opts.SkipValidation

	if opts.Verbose {
		fmt.Printf("Parsing OpenAPI spec: %s\n", opts.InputSpec)
	}

	if err := loadSpec(p, opts.InputSpec); err != nil {
		return nil, err
	}
	if opts.SkipValidation && len(p.ValidationErrors) > 0 {
		fmt.Fprintln(os.Stderr, "There were issues with the specification, but validation has been explicitly disabled.")
		fmt.Fprint(os.Stderr, parser.FormatValidationIssues(p.ValidationErrors, p.ValidationWarnings))
	}

	models, err := p.GetModels()
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}

	operationsByTag, err := p.GetOperations()
	if err != nil {
		return nil, fmt.Errorf("failed to get operations: %w", err)
	}

	resolveOperationIDConflicts(operationsByTag, opts.Verbose)

	securitySchemes, err := p.GetSecuritySchemes()
	if err != nil {
		return nil, fmt.Errorf("failed to get security schemes: %w", err)
	}

	if opts.Verbose {
		fmt.Printf("Found %d models\n", len(models))
		opCount := 0
		for _, ops := range operationsByTag {
			opCount += len(ops)
		}
		fmt.Printf("Found %d operations in %d tags\n", opCount, len(operationsByTag))
	}

	models = cg.PostProcessModels(models)
	for tag, ops := range operationsByTag {
		operationsByTag[tag] = cg.PostProcessOperations(ops)
	}

	return &specData{
		models:          models,
		operationsByTag: operationsByTag,
		securitySchemes: securitySchemes,
		info:            p.GetInfo(),
		basePath:        p.GetBasePath(),
	}, nil
}

// loadSpec loads inputSpec into p, treating an http(s) prefix as a remote
// URL and anything else as a local file path.
func loadSpec(p *parser.Parser, inputSpec string) error {
	if strings.HasPrefix(inputSpec, "http://") || strings.HasPrefix(inputSpec, "https://") {
		if err := p.LoadFromURL(inputSpec); err != nil {
			return fmt.Errorf("failed to load spec from URL: %w", err)
		}
	} else {
		if err := p.LoadFromFile(inputSpec); err != nil {
			return fmt.Errorf("failed to load spec from file: %w", err)
		}
	}
	return nil
}

// resolveOperationIDConflicts detects duplicate operation IDs within each
// tag and renames later occurrences by appending a numeric suffix, so that
// generated method names don't collide.
func resolveOperationIDConflicts(operationsByTag map[string][]*generator.CodegenOperation, verbose bool) {
	for _, tag := range sortedTags(operationsByTag) {
		ops := operationsByTag[tag]
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
}
