package template

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/cbroglie/mustache"
)

// NewEngineFromFS creates a new template engine from an embedded filesystem.
// The subdir parameter specifies the subdirectory within the fs.FS to use as
// the template root (e.g., "typescript-fetch").
func NewEngineFromFS(fsys fs.FS, subdir string) *Engine {
	return &Engine{
		TemplateDir: subdir,
		partials:    make(map[string]string),
		Lambdas:     make(map[string]func(text string, render mustache.RenderFunc) (string, error)),
		fsys:        fsys,
	}
}

// LoadPartialsFromFS loads all partial templates from the embedded filesystem.
func (e *Engine) LoadPartialsFromFS() error {
	if e.fsys == nil {
		return fmt.Errorf("no embedded filesystem configured")
	}

	return fs.WalkDir(e.fsys, e.TemplateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".mustache") {
			return nil
		}

		content, err := fs.ReadFile(e.fsys, path)
		if err != nil {
			return fmt.Errorf("failed to read partial %s: %w", path, err)
		}

		// Get relative name without extension
		// path is like "typescript-fetch/models.mustache"
		// We want just "models" as the partial name
		name := strings.TrimPrefix(path, e.TemplateDir+"/")
		name = strings.TrimSuffix(name, ".mustache")

		e.partials[name] = string(content)
		if e.Verbose {
			fmt.Printf("[TEMPLATE] Loaded embedded partial: %s\n", name)
		}
		return nil
	})
}
