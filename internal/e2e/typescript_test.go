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

	// Each client loads when its suite runs: one whose JavaScript does not
	// even parse fails its own cases, not the program.
	main.WriteString("import { run } from \"./runner\";\n\ndeclare const require: any;\n")

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
	}

	main.WriteString("\nrun({\n")

	for i, s := range suites {
		fmt.Fprintf(&main, "    %q: { load: () => require(\"./c%d/index\"), dir: %q },\n", s.Name, i, filepath.Join(dir, "c"+strconv.Itoa(i)))
	}

	main.WriteString("});\n")
	write(t, filepath.Join(dir, "main.ts"), main.String())
	write(t, filepath.Join(dir, "tsconfig.json"), `{"compilerOptions": {"target": "ES2022", "module": "commonjs", "lib": ["ES2022", "DOM"],
 "outDir": "dist", "strict": true, "skipLibCheck": true, "noEmitOnError": false}, "include": ["**/*.ts"]}`)

	// A client must compile as its users compile it, strict. The JavaScript
	// is emitted anyway, so the others run; a client with a type error fails
	// its cases with the first one.
	build := exec.Command(tsc, "-p", ".")
	build.Dir = dir

	out, _ := build.CombinedOutput()
	broken := map[string]string{}

	for _, line := range strings.Split(string(out), "\n") {
		pkg, _, _ := strings.Cut(line, "/")
		if _, seen := broken[pkg]; !strings.Contains(line, ": error TS") || seen {
			continue
		}

		if !strings.HasPrefix(pkg, "c") {
			t.Fatalf("tsc: the runner does not compile:\n%s", out)
		}

		broken[pkg] = line
	}

	writeInput(t, dir, suites)

	outcomes := execRunner(t, exec.Command(node, filepath.Join(dir, "dist", "main.js"), filepath.Join(dir, "input.json")))

	for i, s := range suites {
		if msg, ok := broken["c"+strconv.Itoa(i)]; ok {
			outcomes[s.Name] = map[int]outcome{}
			for j := range s.Cases {
				outcomes[s.Name][j] = outcome{Suite: s.Name, Case: j, Fail: "the client does not compile: " + msg}
			}
		}
	}

	return outcomes
}
