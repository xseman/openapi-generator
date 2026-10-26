package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xseman/openapi-generator/internal/gen"
)

// runTypeScript generates a typescript-fetch client per suite, compiles
// them with testdata/e2e/runners/ts to CommonJS and runs the suites with
// node.
func runTypeScript(t *testing.T, suites []suite) map[string]map[int]outcome {
	t.Helper()

	tsc, err := exec.LookPath("tsc")
	if err != nil {
		t.Fatal("the typescript-fetch runner compiles with tsc (npm i -g typescript)")
	}

	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("the typescript-fetch runner runs with node 22 or later")
	}

	dir := t.TempDir()

	runner, err := os.ReadFile("../../testdata/e2e/runners/ts/runner.ts")
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(dir, "runner.ts"), string(runner))

	var main strings.Builder

	main.WriteString("import { run } from \"./runner\";\n")

	for i, s := range suites {
		pkg := "c" + strconv.Itoa(i)

		err := gen.Generate(gen.Options{
			InputSpec:      s.Spec,
			OutputDir:      filepath.Join(dir, pkg),
			GeneratorName:  "typescript-fetch",
			TemplateDir:    "../../templates/typescript-fetch",
			SkipValidation: true,
		})
		if err != nil {
			t.Fatalf("generate %s: %v", s.Name, err)
		}

		fmt.Fprintf(&main, "import * as %s from \"./%s/index\";\n", pkg, pkg)
	}

	main.WriteString("\nrun({\n")

	for i, s := range suites {
		fmt.Fprintf(&main, "    %q: { module: c%d, dir: %q },\n", s.Name, i, filepath.Join(dir, "c"+strconv.Itoa(i)))
	}

	main.WriteString("});\n")
	write(t, filepath.Join(dir, "main.ts"), main.String())
	write(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions": {"target": "ES2022", "module": "commonjs", "lib": ["ES2022", "DOM"],
 "outDir": "dist", "strict": false, "skipLibCheck": true, "noEmitOnError": false}, "files": ["main.ts"]}`)

	// Type errors in a client are its generator's; the JavaScript is
	// emitted anyway and the cases show what they break.
	build := exec.Command(tsc, "-p", ".")
	build.Dir = dir

	if out, err := build.CombinedOutput(); err != nil {
		t.Logf("tsc: %v\n%s", err, out)
	}

	writeInput(t, dir, suites)

	return execRunner(t, exec.Command(node, filepath.Join(dir, "dist", "main.js"), filepath.Join(dir, "input.json")))
}
