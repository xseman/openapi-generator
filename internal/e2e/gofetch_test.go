package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xseman/openapi-generator/internal/gen"
)

// suite is one scenario as a runner runs it: where to send, the cases, and
// for each the names its args use.
type suite struct {
	Name  string      `json:"name"`
	Base  string      `json:"base"`
	Mock  bool        `json:"mock"`
	Auth  *auth       `json:"auth,omitempty"`
	Cases []suiteCase `json:"cases"`
	Spec  string      `json:"-"`

	// Schemes are the spec's security schemes by name and type (apiKey,
	// http, oauth2, openIdConnect), for a client whose token function
	// depends on it.
	Schemes map[string]string `json:"schemes"`
}

type suiteCase struct {
	Op     string            `json:"op"`
	Params []string          `json:"params"`
	Bodies []string          `json:"bodies,omitempty"`
	Args   json.RawMessage   `json:"args,omitempty"`
	Auth   *auth             `json:"auth,omitempty"`
	Save   map[string]string `json:"save,omitempty"`
}

// outcome is what a runner reports for a case: the result as wire JSON,
// the *ResponseError, or why the call itself failed.
type outcome struct {
	Suite  string          `json:"suite"`
	Case   int             `json:"case"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Status int             `json:"status"`
		Model  json.RawMessage `json:"model"`
		Body   string          `json:"body"`
	} `json:"error"`
	Fail string `json:"fail"`
}

// runGoFetch generates a go-fetch client per suite into one module, builds
// testdata/e2e/runners/go with a main that hands it those clients, runs the
// suites and returns the outcomes by suite name and case.
func runGoFetch(t *testing.T, suites []suite) map[string]map[int]outcome {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	write(t, filepath.Join(dir, "go.mod"), "module e2e\n\ngo 1.24\n")

	runner, err := os.ReadFile("../../testdata/e2e/runners/go/runner.go")
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(dir, "runner.go"), string(runner))

	var imports, setups strings.Builder

	for i, s := range suites {
		pkg := "c" + strconv.Itoa(i)

		err := gen.Generate(gen.Options{
			InputSpec:            s.Spec,
			OutputDir:            filepath.Join(dir, pkg),
			GeneratorName:        "go-fetch",
			TemplateDir:          "../../templates/go-fetch",
			AdditionalProperties: []string{"packageName=" + pkg},
			SkipValidation:       true,
		})
		if err != nil {
			t.Fatalf("generate %s: %v", s.Name, err)
		}

		fmt.Fprintf(&imports, "\t%s \"e2e/%s\"\n", pkg, pkg)
		fmt.Fprintf(&setups, "\t\t%q: func(base string, a *Auth) ([]any, any) {\n", s.Name)
		fmt.Fprintf(&setups, "\t\t\tcfg := &%s.Configuration{BasePath: base, APIKey: func(context.Context, string, string) (string, error) { return a.APIKey, nil }, "+
			"AccessToken: func(context.Context, string, string, []string) (string, error) { return a.Bearer, nil }}\n\n", pkg)
		fmt.Fprintf(&setups, "\t\t\treturn []any{%s}, cfg\n\t\t},\n", constructors(t, filepath.Join(dir, pkg), pkg))
	}

	write(t, filepath.Join(dir, "main.go"), "package main\n\nimport (\n\t\"context\"\n\n"+imports.String()+
		")\n\nfunc main() {\n\trun(map[string]setup{\n"+setups.String()+"\t})\n}\n")

	build := exec.Command(goBin, "build", "-o", "runner", ".")
	build.Dir = dir

	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")

	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build runner: %v\n%s", err, out)
	}

	writeInput(t, dir, suites)

	return execRunner(t, exec.Command(filepath.Join(dir, "runner"), filepath.Join(dir, "input.json")))
}

// execRunner runs a built runner and reads its outcome lines. A runner that
// dies leaves the cases after it without an outcome; the ones before still
// count.
func execRunner(t *testing.T, cmd *exec.Cmd) map[string]map[int]outcome {
	t.Helper()

	var stderr bytes.Buffer

	// Away from UTC, so a client that reads or writes local time shows it.
	cmd.Env = append(os.Environ(), "TZ=Europe/Bratislava")
	cmd.Stderr = &stderr

	stdout, err := cmd.Output()
	if err != nil {
		t.Errorf("runner: %v\n%s", err, stderr.String())
	}

	out := map[string]map[int]outcome{}

	sc := bufio.NewScanner(bytes.NewReader(stdout))
	sc.Buffer(nil, 1<<26)

	for sc.Scan() {
		var o outcome
		if err := json.Unmarshal(sc.Bytes(), &o); err != nil {
			t.Fatalf("runner output %q: %v", sc.Text(), err)
		}

		if out[o.Suite] == nil {
			out[o.Suite] = map[int]outcome{}
		}

		out[o.Suite][o.Case] = o
	}

	return out
}

// constructors lists the New<Tag>API(cfg) calls of a generated package.
func constructors(t *testing.T, dir, pkg string) string {
	t.Helper()

	files, _ := filepath.Glob(filepath.Join(dir, "api_*.go"))

	var calls []string

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}

		for _, line := range strings.Split(string(data), "\n") {
			if name, ok := strings.CutPrefix(line, "func New"); ok {
				calls = append(calls, pkg+".New"+name[:strings.IndexByte(name, '(')]+"(cfg)")
			}
		}
	}

	return strings.Join(calls, ", ")
}

// writeInput writes the suites as the runner's input.json in dir.
func writeInput(t *testing.T, dir string, suites []suite) {
	t.Helper()

	in, err := json.Marshal(map[string]any{"suites": suites})
	if err != nil {
		t.Fatal(err)
	}

	write(t, filepath.Join(dir, "input.json"), string(in))
}

func write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
