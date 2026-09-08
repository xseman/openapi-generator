package gen

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/xseman/openapi-generator/internal/generator"
)

// writeIndexFiles writes the per-folder barrel (index) file for models, and
// for APIs, when there is at least one model or operation respectively.
func (r *renderer) writeIndexFiles(models []*generator.CodegenModel, operationsByTag map[string][]*generator.CodegenOperation) ([]string, error) {
	indexExt := ".ts"
	if r.dartGen != nil {
		indexExt = ".dart"
	}

	var generatedFiles []string

	if len(models) > 0 {
		var modelIndex string
		switch {
		case r.tsGen != nil:
			modelIndex = generateModelIndex(models, r.tsGen)
		case r.dartGen != nil:
			modelIndex = generateDartModelIndex(models, r.dartGen)
		}
		indexFilename := "index" + indexExt
		if r.dartGen != nil {
			indexFilename = "models.dart"
		}
		modelIndexPath := filepath.Join(r.outputDir, r.modelPackage, indexFilename)
		if err := os.MkdirAll(filepath.Dir(modelIndexPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create model index directory: %w", err)
		}
		if err := os.WriteFile(modelIndexPath, []byte(modelIndex), 0600); err != nil {
			return nil, fmt.Errorf("failed to write model index: %w", err)
		}
		generatedFiles = append(generatedFiles, filepath.Join(r.modelPackage, indexFilename))
	}

	if len(operationsByTag) > 0 {
		var apiIndex string
		switch {
		case r.tsGen != nil:
			apiIndex = generateApiIndex(operationsByTag, r.tsGen)
		case r.dartGen != nil:
			apiIndex = generateDartApiIndex(operationsByTag, r.dartGen)
		}
		indexFilename := "index" + indexExt
		if r.dartGen != nil {
			indexFilename = "apis.dart"
		}
		apiIndexPath := filepath.Join(r.outputDir, r.apiPackage, indexFilename)
		if err := os.MkdirAll(filepath.Dir(apiIndexPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create API index directory: %w", err)
		}
		if err := os.WriteFile(apiIndexPath, []byte(apiIndex), 0600); err != nil {
			return nil, fmt.Errorf("failed to write API index: %w", err)
		}
		generatedFiles = append(generatedFiles, filepath.Join(r.apiPackage, indexFilename))
	}

	return generatedFiles, nil
}

// resolveDiscriminatorFilenames fills in the generator-specific file name
// for each discriminator-mapped model, so template imports point at the
// actual generated files.
func resolveDiscriminatorFilenames(models []*generator.CodegenModel, cg generator.CodegenConfig) {
	for _, model := range models {
		if model.Discriminator == nil {
			continue
		}
		for _, mapped := range model.Discriminator.MappedModels {
			mapped.ModelFilename = cg.ToModelFilename(mapped.ModelName)
		}
	}
}
