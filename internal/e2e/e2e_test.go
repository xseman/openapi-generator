package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// runner runs suites through the clients of one generator, named as
// scenarios name it in skip, and returns the outcomes by suite and case.
type runner struct {
	generator string
	run       func(t *testing.T, suites []suite) map[string]map[int]outcome
}

var runners = []runner{
	{"go-fetch", runGoFetch},
	{"typescript-fetch", runTypeScript},
	{"dart-fetch", runDart},
}

// forEachRunner runs fn per generator; E2E_GENERATOR=<name> picks one.
func forEachRunner(t *testing.T, fn func(t *testing.T, r runner)) {
	t.Helper()

	for _, r := range runners {
		if only := os.Getenv("E2E_GENERATOR"); only != "" && only != r.generator {
			continue
		}

		t.Run(r.generator, func(t *testing.T) { fn(t, r) })
	}
}

// requireE2E skips unless the run asks for the e2e tests: they need uv,
// build clients and start servers.
func requireE2E(t *testing.T) {
	t.Helper()

	if os.Getenv("E2E") == "" {
		t.Skip("set E2E=1 (make e2e) to run the e2e tests")
	}
}

// TestMock runs the scenarios of the feature specs against the mock: each
// request must pass the spec's validation and look as the case says, and
// the client must return what the case says from the mock's response.
func TestMock(t *testing.T) {
	requireE2E(t)

	scenarios := loadScenarios(t)

	forEachRunner(t, func(t *testing.T, r runner) {
		t.Helper()

		var (
			suites []suite
			mocks  = map[string]*mock{}
			byName = map[string]*scenario{}
		)

		for _, s := range scenarios {
			if s.Spec == "" {
				continue
			}

			m := newMock(t, s.Spec, s.Cases)
			mocks[s.Name], byName[s.Name] = m, s
			suites = append(suites, toSuite(t, s, s.Spec, m.srv.URL, true))
		}

		outcomes := r.run(t, suites)
		tally := map[string]*counts{}

		for _, s := range suites {
			t.Run(s.Name, func(t *testing.T) {
				tally[s.Name] = checkCases(t, r.generator, byName[s.Name], outcomes[s.Name], mocks[s.Name])
			})
		}

		report(t, r.generator, tally)
	})
}

// counts are a suite's cases by how they ended.
type counts struct{ pass, fail, skip int }

// report logs a PASS/FAIL/SKIP table of the suites for the generator.
func report(t *testing.T, generator string, tally map[string]*counts) {
	t.Helper()

	var b strings.Builder

	fmt.Fprintf(&b, "\n%-32s %5s %5s %5s  (%s)\n", "scenario", "pass", "fail", "skip", generator)

	for _, name := range slices.Sorted(maps.Keys(tally)) {
		c := tally[name]
		fmt.Fprintf(&b, "%-32s %5d %5d %5d\n", name, c.pass, c.fail, c.skip)
	}

	t.Log(b.String())
}

// TestFastAPI starts the FastAPI backend, checks its spec against the
// snapshot (E2E_UPDATE=1 rewrites it), generates the client from the spec
// the server serves and runs the backend's scenario against it.
func TestFastAPI(t *testing.T) {
	requireE2E(t)

	uv, err := exec.LookPath("uv")
	if err != nil {
		t.Fatal("the FastAPI backend runs with uv: https://docs.astral.sh/uv/")
	}

	scenarios := loadScenarios(t)

	// A fresh backend per generator: the cases share its state.
	forEachRunner(t, func(t *testing.T, r runner) {
		t.Helper()

		base := startBackend(t, uv, "../../testdata/e2e/backends/fastapi")

		resp, err := http.Get(base + "/openapi.json")
		if err != nil {
			t.Fatal(err)
		}

		defer func() { _ = resp.Body.Close() }()

		var served any
		if err := json.NewDecoder(resp.Body).Decode(&served); err != nil {
			t.Fatal(err)
		}

		spec := checkSnapshot(t, served, "../../testdata/e2e/specs/fastapi.json")

		for _, s := range scenarios {
			if s.Backend != "fastapi" {
				continue
			}

			outcomes := r.run(t, []suite{toSuite(t, s, spec, base, false)})

			var c *counts

			t.Run(s.Name, func(t *testing.T) { c = checkCases(t, r.generator, s, outcomes[s.Name], nil) })
			report(t, r.generator, map[string]*counts{s.Name: c})
		}
	})
}

func toSuite(t *testing.T, s *scenario, specPath, base string, isMock bool) suite {
	t.Helper()

	doc, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		t.Fatal(err)
	}

	params := opParams(doc)

	out := suite{Name: s.Name, Base: base, Mock: isMock, Auth: s.Auth, Spec: specPath, Schemes: map[string]string{}}

	if doc.Components != nil {
		for name, ref := range doc.Components.SecuritySchemes {
			if ref.Value != nil {
				out.Schemes[name] = ref.Value.Type
			}
		}
	}

	for _, k := range s.Cases {
		if _, ok := params[k.Op]; !ok {
			t.Fatalf("%s: no operation %s in %s", s.Name, k.Op, specPath)
		}

		if !isMock && (k.Request != nil || k.Response != nil) {
			t.Fatalf("%s %s: request and response are for the mock; a backend answers itself", s.Name, k.Op)
		}

		out.Cases = append(out.Cases, suiteCase{Op: k.Op, Params: params[k.Op], Args: k.Args, Auth: k.Auth, Save: k.Save})
	}

	return out
}

// checkCases compares each case's outcome, and with a mock the request it
// received, with the scenario. A case skipped for this generator must still
// fail: one that passes has its skip left over.
func checkCases(t *testing.T, generator string, s *scenario, outcomes map[int]outcome, m *mock) *counts {
	t.Helper()

	c := &counts{}

	for i, k := range s.Cases {
		name := k.Name
		if name == "" {
			name = k.Op
		}

		t.Run(fmt.Sprintf("%02d %s", i, name), func(t *testing.T) {
			var diffs []string

			if o, ok := outcomes[i]; ok {
				diffs = caseDiffs(k, o, m, i)
			} else {
				diffs = []string{"the runner reported nothing for this case"}
			}

			reason, skipped := k.Skip[generator]
			if !skipped {
				reason, skipped = s.Skip[generator]
			}

			switch {
			case skipped && len(diffs) == 0:
				c.fail++

				t.Errorf("passes now: drop its skip (%s)", reason)

			case skipped:
				c.skip++

				t.Skipf("%s (%s)", reason, strings.Join(diffs, "; "))

			case len(diffs) > 0:
				c.fail++

				t.Error("\n" + strings.Join(diffs, "\n"))

			default:
				c.pass++
			}
		})
	}

	return c
}

func caseDiffs(k kase, o outcome, m *mock, i int) []string {
	var diffs []string

	// With a mock every case's request must be valid for the spec, and the
	// scenario's response too; "request" checks more of it.
	if m != nil {
		m.mu.Lock()
		got, ok := m.received[i]
		m.mu.Unlock()

		switch {
		case !ok:
			diffs = append(diffs, "request: none reached the server")
		case got["invalid"] != nil:
			diffs = append(diffs, fmt.Sprintf("request: invalid for the spec: %v", got["invalid"]))
		case got["responseInvalid"] != nil:
			diffs = append(diffs, fmt.Sprintf("response: the scenario's is invalid for the spec: %v", got["responseInvalid"]))
		case k.Request != nil:
			diffs = append(diffs, match("request", decodeJSON(k.Request), got)...)
		}
	}

	switch {
	case o.Fail != "":
		diffs = append(diffs, "call failed: "+o.Fail)
	case k.Error != nil:
		if o.Error == nil {
			diffs = append(diffs, "error: want one, got result "+string(o.Result))

			break
		}

		got := map[string]any{"status": json.Number(strconv.Itoa(o.Error.Status)), "body": o.Error.Body, "model": nil}
		if len(o.Error.Model) > 0 {
			got["model"] = decodeJSON(o.Error.Model)
		}

		diffs = append(diffs, match("error", decodeJSON(k.Error), got)...)

	case o.Error != nil:
		diffs = append(diffs, fmt.Sprintf("error: %d %s", o.Error.Status, o.Error.Body))
	case k.Result != nil:
		diffs = append(diffs, match("result", decodeJSON(k.Result), decodeJSON(o.Result))...)
	}

	return diffs
}

// startBackend runs `uv run uvicorn app:app` in dir on a free port and
// returns its URL once it answers; the test stops it.
func startBackend(t *testing.T, uv, dir string) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	addr, _ := l.Addr().(*net.TCPAddr)
	port := strconv.Itoa(addr.Port)
	_ = l.Close()

	cmd := exec.Command(uv, "run", "--quiet", "--locked", "uvicorn", "app:app", "--port", port, "--log-level", "warning")
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	exited := make(chan error, 1)

	go func() { exited <- cmd.Wait() }()

	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)

		<-exited
	})

	base := "http://127.0.0.1:" + port

	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); {
		select {
		case err := <-exited:
			exited <- err

			t.Fatalf("the backend exited: %v", err)

		case <-time.After(200 * time.Millisecond):
		}

		if resp, err := http.Get(base + "/openapi.json"); err == nil {
			_ = resp.Body.Close()

			return base
		}
	}

	t.Fatal("the backend did not start in 2 minutes")

	return ""
}

// checkSnapshot compares the served spec with the committed one and returns
// the path of the served spec, written next to the test's temp files.
func checkSnapshot(t *testing.T, served any, snapshot string) string {
	t.Helper()

	data, err := json.MarshalIndent(served, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	data = append(data, '\n')

	if os.Getenv("E2E_UPDATE") != "" {
		if err := os.WriteFile(snapshot, data, 0o644); err != nil { //nolint:gosec // a file in the repository
			t.Fatal(err)
		}
	} else if old, err := os.ReadFile(snapshot); err != nil || !bytes.Equal(old, data) {
		t.Errorf("the backend's spec differs from %s; rerun with `make e2e-update` and review the diff", snapshot)
	}

	path := t.TempDir() + "/openapi.json"
	write(t, path, string(data))

	return path
}
