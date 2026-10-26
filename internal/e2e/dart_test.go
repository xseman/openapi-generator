package e2e

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xseman/openapi-generator/internal/gen"
)

// dartVersion reads "Dart SDK version: 3.13.7 ..." as 3 and 13.
var dartVersion = regexp.MustCompile(`version: (\d+)\.(\d+)`)

// runDart generates a dart-fetch package per suite, depends on them from a
// package holding testdata/e2e/runners/dart and runs the suites on the VM.
// E2E_DART names the dart binary; the clients need Dart 3.13 or later.
func runDart(t *testing.T, suites []suite) map[string]map[int]outcome {
	t.Helper()

	dart := os.Getenv("E2E_DART")
	if dart == "" {
		dart = "dart"
	}

	ver, err := exec.Command(dart, "--version").CombinedOutput()
	if m := dartVersion.FindStringSubmatch(string(ver)); err != nil || m == nil || m[1] == "3" && atoi(m[2]) < 13 {
		t.Skipf("dart-fetch clients need Dart 3.13 or later (set E2E_DART): %s %v", strings.TrimSpace(string(ver)), err)
	}

	dir := t.TempDir()

	runner, err := os.ReadFile("../../testdata/e2e/runners/dart/runner.dart")
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(dir, "runner.dart"), string(runner))

	for i, s := range suites {
		err := gen.Generate(gen.Options{
			InputSpec:            s.Spec,
			OutputDir:            filepath.Join(dir, "c"+strconv.Itoa(i)),
			GeneratorName:        "dart-fetch",
			TemplateDir:          "../../templates/dart-fetch",
			AdditionalProperties: []string{"pubName=c" + strconv.Itoa(i)},
			SkipValidation:       true,
		})
		if err != nil {
			t.Fatalf("generate %s: %v", s.Name, err)
		}
	}

	// A client that does not compile would stop the whole program: it
	// leaves the run, and its cases fail with its first error.
	broken := dartErrors(t, dart, dir)
	failed := map[string]map[int]outcome{}

	var imports, clients, deps strings.Builder

	for i, s := range suites {
		pkg := "c" + strconv.Itoa(i)

		if msg, ok := broken[pkg]; ok {
			failed[s.Name] = map[int]outcome{}
			for j := range s.Cases {
				failed[s.Name][j] = outcome{Suite: s.Name, Case: j, Fail: "the client does not compile: " + msg}
			}

			continue
		}

		fmt.Fprintf(&deps, "  %s:\n    path: %s\n", pkg, pkg)
		fmt.Fprintf(&imports, "import 'package:%s/%s.dart' as %s;\n", pkg, pkg, pkg)
		fmt.Fprintf(&clients, "    %q: Client(\n      (base, a) => %s.Configuration(basePath: base, username: a.username, password: a.password, "+
			"apiKey: (name) => a.apiKey ?? '', accessToken: () => a.bearer ?? ''),\n      (cfg) => [%s],\n      %q),\n",
			s.Name, pkg, dartAPIs(t, filepath.Join(dir, pkg), pkg), pkg)
	}

	var running []suite

	for _, s := range suites {
		if _, ok := failed[s.Name]; !ok {
			running = append(running, s)
		}
	}

	if len(running) == 0 {
		return failed
	}

	write(t, filepath.Join(dir, "pubspec.yaml"), "name: e2e_runner\npublish_to: none\nenvironment:\n  sdk: '>=3.13.0 <4.0.0'\ndependencies:\n"+deps.String())
	write(t, filepath.Join(dir, "main.dart"), "import 'runner.dart';\n"+imports.String()+
		"\nvoid main(List<String> args) => run(args[0], {\n"+clients.String()+"});\n")

	get := exec.Command(dart, "pub", "get", "--offline")
	get.Dir = dir

	if out, err := get.CombinedOutput(); err != nil {
		t.Fatalf("dart pub get: %v\n%s", err, out)
	}

	writeInput(t, dir, running)

	cmd := exec.Command(dart, "run", "main.dart", filepath.Join(dir, "input.json"))
	cmd.Dir = dir

	out := execRunner(t, cmd)
	maps.Copy(out, failed)

	return out
}

// dartErrors analyzes the generated packages under dir and returns the
// first compile error of each package that has one.
func dartErrors(t *testing.T, dart, dir string) map[string]string {
	t.Helper()

	cmd := exec.Command(dart, "analyze", "--format=machine", ".")
	cmd.Dir = dir

	out, _ := cmd.CombinedOutput()

	errs := map[string]string{}

	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(line, "|")
		if len(f) < 8 || f[0] != "ERROR" {
			continue
		}

		rel, err := filepath.Rel(dir, f[3])
		if err != nil {
			continue
		}

		pkg, _, _ := strings.Cut(rel, string(filepath.Separator))
		if _, seen := errs[pkg]; !seen {
			errs[pkg] = fmt.Sprintf("%s:%s: %s", rel, f[4], f[7])
		}
	}

	return errs
}

// dartAPIs lists the constructors of a package's API classes for cfg.
func dartAPIs(t *testing.T, dir, pkg string) string {
	t.Helper()

	files, _ := filepath.Glob(filepath.Join(dir, "lib", "src", "apis", "*.dart"))
	class := regexp.MustCompile(`(?m)^class (\w+) extends BaseApi`)

	var calls []string

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range class.FindAllStringSubmatch(string(data), -1) {
			calls = append(calls, pkg+"."+m[1]+"(cfg as "+pkg+".Configuration)")
		}
	}

	return strings.Join(calls, ", ")
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)

	return n
}
