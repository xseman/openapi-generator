package gen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// generateMetadata creates the .openapi-generator folder with FILES and
// VERSION, mirroring the metadata the Java openapi-generator writes.
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
