package gen

import (
	"fmt"

	"github.com/xseman/openapi-generator/internal/config"
)

// applyGoFetchAdditionalProperties maps CLI / config additional-property
// values onto a GoFetchConfig.
func applyGoFetchAdditionalProperties(gc *config.GoFetchConfig, props map[string]any) {
	if v, ok := props["packageName"].(string); ok {
		gc.PackageName = v
	}

	if v, ok := props["withGoMod"].(bool); ok {
		gc.WithGoMod = v
	}

	if v, ok := props["moduleName"].(string); ok {
		gc.ModuleName = v
	}
}

// goDeclareParams marks the operations whose Params type this API file
// declares: an operation under several tags is rendered into each tag's
// file, and its Params type must be declared in only one, the first.
func goDeclareParams(opMaps []map[string]any, declared map[string]bool) {
	for _, op := range opMaps {
		ext, _ := op["vendorExtensions"].(map[string]any)
		name, _ := ext["x-go-params"].(string)

		op["goDeclareParams"] = !declared[name]
		declared[name] = true
	}
}

// PrintGoFetchConfigHelp prints the go-fetch additional config options to
// stdout (used by the "config-help" CLI command).
func PrintGoFetchConfigHelp() {
	fmt.Println("CONFIG OPTIONS for go-fetch:")
	fmt.Println()
	fmt.Println("  packageName")
	fmt.Println("      Go package name of the generated files. (Default: openapi)")
	fmt.Println()
	fmt.Println("  withGoMod")
	fmt.Println("      Write a go.mod, making the output a module of its own.")
	fmt.Println("      (Default: false)")
	fmt.Println()
	fmt.Println("  moduleName")
	fmt.Println("      Module path in go.mod. (Default: packageName)")
	fmt.Println()
}
