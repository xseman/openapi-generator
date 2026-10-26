package gen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// goTool returns the go binary, skipping the test under -short or without
// one: these tests compile generated code.
func goTool(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("compiles generated Go code")
	}

	bin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go in PATH")
	}

	return bin
}

// runGo runs the go tool in dir and fails the test with its output when it
// fails or finds no package.
func runGo(t *testing.T, bin, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir

	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")

	out, err := cmd.CombinedOutput()
	if err != nil || bytes.Contains(out, []byte("matched no packages")) {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func generateGoFetch(t *testing.T, spec, out string, props ...string) {
	t.Helper()

	err := Generate(Options{
		InputSpec:            spec,
		OutputDir:            out,
		GeneratorName:        "go-fetch",
		TemplateDir:          filepath.Join("..", "..", "templates", "go-fetch"),
		AdditionalProperties: props,
		SkipValidation:       true,
	})
	if err != nil {
		t.Fatalf("Generate %s: %v", spec, err)
	}
}

// TestGoFetchCompiles generates every feature spec into a package of one
// module, vets it and runs the tests testdata/gofetch/<spec>/ holds, which
// exercise the client against httptest.
func TestGoFetchCompiles(t *testing.T) {
	bin := goTool(t)
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module gofetchcheck\n\ngo 1.24\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	specs, err := filepath.Glob(filepath.Join("..", "..", "testdata", "specs", "feature", "*.yaml"))
	if err != nil || len(specs) == 0 {
		t.Fatalf("no feature specs: %v", err)
	}

	for _, spec := range specs {
		name := strings.TrimSuffix(filepath.Base(spec), ".yaml")
		out := filepath.Join(root, name)
		generateGoFetch(t, spec, out, "packageName="+strings.ReplaceAll(name, "-", ""))

		tests, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "gofetch", name, "*_test.go"))
		for _, test := range tests {
			data, err := os.ReadFile(test)
			if err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(filepath.Join(out, filepath.Base(test)), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}

	// go test builds no package without a test file: vet compiles them all.
	runGo(t, bin, root, "vet", "./...")
	runGo(t, bin, root, "test", "./...")
}

// TestGoFetchGoMod checks withGoMod on its own: a go.mod inside the module
// of TestGoFetchCompiles would cut its directory out of ./....
func TestGoFetchGoMod(t *testing.T) {
	bin := goTool(t)
	out := t.TempDir()
	generateGoFetch(t, filepath.Join("..", "..", "testdata", "specs", "feature", "request-body-json.yaml"), out, "withGoMod=true")

	data, err := os.ReadFile(filepath.Join(out, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}

	if want := "module openapi\n\ngo 1.24\n"; string(data) != want {
		t.Errorf("go.mod = %q, want %q", data, want)
	}

	runGo(t, bin, out, "vet", "./...")
}
