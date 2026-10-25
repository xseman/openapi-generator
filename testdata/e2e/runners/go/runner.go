// Command runner calls the operations of generated go-fetch clients as the
// e2e scenarios say and prints one JSON line per case. It knows nothing of
// a particular client: main.go, written next to it for each run, hands it
// the API values and their Configuration, and it finds methods, parameters
// and results by reflection. Standard library only.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Auth is what a case authenticates with; the Configuration funcs read it
// at call time.
type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	APIKey   string `json:"apiKey"`
	Bearer   string `json:"bearer"`
}

type input struct {
	Suites []suite `json:"suites"`
}

type suite struct {
	Name  string `json:"name"`
	Base  string `json:"base"`
	Mock  bool   `json:"mock"`
	Auth  Auth   `json:"auth"`
	Cases []cas  `json:"cases"`
}

type cas struct {
	Op     string            `json:"op"`
	Params []string          `json:"params"`
	Args   json.RawMessage   `json:"args"`
	Auth   *Auth             `json:"auth"`
	Save   map[string]string `json:"save"`
}

type outcome struct {
	Suite  string          `json:"suite"`
	Case   int             `json:"case"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *respErr        `json:"error,omitempty"`
	Fail   string          `json:"fail,omitempty"`
}

type respErr struct {
	Status int             `json:"status"`
	Model  json.RawMessage `json:"model,omitempty"`
	Body   string          `json:"body"`
}

// setup builds the API values of one suite's client, sending to base, and
// returns them with the *Configuration they share.
type setup func(base string, auth *Auth) (apis []any, cfg any)

func run(setups map[string]setup) {
	var in input

	data, err := os.ReadFile(os.Args[1])
	if err == nil {
		err = json.Unmarshal(data, &in)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	out := json.NewEncoder(os.Stdout)

	for _, s := range in.Suites {
		auth := s.Auth
		apis, cfg := setups[s.Name](s.Base, &auth)
		vars := map[string]string{}

		for i, c := range s.Cases {
			o := outcome{Suite: s.Name, Case: i}

			if c.Auth != nil {
				auth = merge(auth, *c.Auth)
			}

			auth = expandAuth(auth, vars)
			setBasic(cfg, auth)

			if s.Mock {
				mark(s.Base, i)
			}

			callCase(&o, apis, c, vars)
			_ = out.Encode(o)
		}
	}
}

// mark tells the mock server which case the next requests belong to.
func mark(base string, i int) {
	resp, err := http.Post(base+"/__e2e/case/"+strconv.Itoa(i), "text/plain", nil)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func callCase(o *outcome, apis []any, c cas, vars map[string]string) {
	// A panic in reflection or in the client fails this case, not the run.
	defer func() {
		if r := recover(); r != nil {
			o.Result, o.Error, o.Fail = nil, nil, fmt.Sprint("panic: ", r)
		}
	}()

	method, ok := findMethod(apis, c.Op)
	if !ok {
		o.Fail = "no method for operation " + c.Op

		return
	}

	args := map[string]json.RawMessage{}
	if len(c.Args) > 0 {
		expanded, err := expand(string(c.Args), vars, true)
		if err == nil {
			err = json.Unmarshal([]byte(expanded), &args)
		}

		if err != nil {
			o.Fail = "args: " + err.Error()

			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	in := []reflect.Value{reflect.ValueOf(ctx)}

	if t := method.Type(); t.NumIn() > 1 && t.In(1).Kind() == reflect.Struct {
		p, err := params(t.In(1), c.Params, args)
		if err != nil {
			o.Fail = err.Error()

			return
		}

		in = append(in, p)
	}

	res := method.Call(in)

	if err, _ := res[len(res)-1].Interface().(error); err != nil {
		o.Error, o.Fail = responseError(err)

		return
	}

	if len(res) == 1 {
		o.Result = json.RawMessage("null")

		return
	}

	b, err := json.Marshal(res[0].Interface())
	if err != nil {
		o.Fail = "result: " + err.Error()

		return
	}

	o.Result = b
	save(b, c.Save, vars)
}

// findMethod finds the typed method of an operation: its operationId with
// the punctuation dropped, with "Method" after it first, the name a
// generator gives a FooRaw next to Foo (whose own FooRaw would match too).
func findMethod(apis []any, op string) (reflect.Value, bool) {
	for _, want := range []string{norm(op) + "method", norm(op)} {
		for _, api := range apis {
			v := reflect.ValueOf(api)
			for i := range v.NumMethod() {
				if norm(v.Type().Method(i).Name) == want {
					return v.Method(i), true
				}
			}
		}
	}

	return reflect.Value{}, false
}

// params fills an operation's Params struct from the args, keyed by the
// spec's names. The field no spec name matches is the body, args["body"].
func params(t reflect.Type, names []string, args map[string]json.RawMessage) (reflect.Value, error) {
	p := reflect.New(t).Elem()

	byField := map[string]string{}
	for _, name := range names {
		byField[numbered(byField, norm(name))] = name
	}

	used := map[string]bool{}

	for i := range t.NumField() {
		f := t.Field(i)

		name, isParam := byField[norm(f.Name)]
		if !isParam {
			name = "body"
		}

		raw, ok := args[name]
		if !ok {
			continue
		}

		if used[name] {
			return p, fmt.Errorf("arg %s: fits fields %s and another", name, f.Name)
		}

		used[name] = true

		if err := set(p.Field(i), raw); err != nil {
			return p, fmt.Errorf("arg %s: %w", name, err)
		}
	}

	for name := range args {
		if !used[name] {
			return p, fmt.Errorf("arg %s: no field of %s takes it", name, t.Name())
		}
	}

	return p, nil
}

// fileArg is a file or raw body in the args: {"$file": {...}} or
// {"$bytes": {...}}, its content as text or base64.
type fileArg struct {
	File  *content `json:"$file"`
	Bytes *content `json:"$bytes"`
}

type content struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Base64 string `json:"base64"`
}

func (c content) data() []byte {
	if c.Base64 != "" {
		b, _ := base64.StdEncoding.DecodeString(c.Base64)

		return b
	}

	return []byte(c.Text)
}

var readerType = reflect.TypeFor[io.Reader]()

// set decodes raw into the field: JSON for every type, files and readers
// from their fileArg.
func set(f reflect.Value, raw json.RawMessage) error {
	var fa fileArg
	_ = json.Unmarshal(raw, &fa)

	switch {
	case f.Type() == readerType && fa.Bytes != nil:
		f.Set(reflect.ValueOf(io.Reader(bytes.NewReader(fa.Bytes.data()))))

		return nil
	case isFile(f.Type()):
		return setFile(f, raw)
	}

	return json.Unmarshal(raw, f.Addr().Interface())
}

// isFile reports whether t is a client's File, *File or []File: a struct
// of Name and Body.
func isFile(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct || t.NumField() != 2 {
		return false
	}

	n, nok := t.FieldByName("Name")
	b, bok := t.FieldByName("Body")

	return nok && bok && n.Type.Kind() == reflect.String && b.Type == readerType
}

func setFile(f reflect.Value, raw json.RawMessage) error {
	switch f.Kind() {
	case reflect.Slice:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return err
		}

		s := reflect.MakeSlice(f.Type(), len(items), len(items))
		for i, item := range items {
			if err := setFile(s.Index(i), item); err != nil {
				return err
			}
		}

		f.Set(s)

		return nil
	case reflect.Pointer:
		v := reflect.New(f.Type().Elem())
		if err := setFile(v.Elem(), raw); err != nil {
			return err
		}

		f.Set(v)

		return nil
	}

	var fa fileArg
	if err := json.Unmarshal(raw, &fa); err != nil || fa.File == nil {
		return fmt.Errorf("want {\"$file\": ...}, got %s", raw)
	}

	f.FieldByName("Name").SetString(fa.File.Name)
	f.FieldByName("Body").Set(reflect.ValueOf(io.Reader(bytes.NewReader(fa.File.data()))))

	return nil
}

// responseError reads a client's *ResponseError by its fields; any other
// error is a failure of the call itself.
func responseError(err error) (*respErr, string) {
	for e := err; e != nil; e = errors.Unwrap(e) {
		v := reflect.ValueOf(e)
		if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
			continue
		}

		s := v.Elem()
		status, body, model := s.FieldByName("StatusCode"), s.FieldByName("Body"), s.FieldByName("Model")

		if !status.IsValid() || !body.IsValid() || !model.IsValid() {
			continue
		}

		r := &respErr{Status: int(status.Int()), Body: string(body.Bytes())}
		if !model.IsNil() {
			r.Model, _ = json.Marshal(model.Interface())
		}

		return r, ""
	}

	return nil, err.Error()
}

// setBasic sets the basic credentials on the *Configuration, which holds
// them as values.
func setBasic(cfg any, a Auth) {
	v := reflect.ValueOf(cfg).Elem()
	if f := v.FieldByName("Username"); f.IsValid() {
		f.SetString(a.Username)
	}

	if f := v.FieldByName("Password"); f.IsValid() {
		f.SetString(a.Password)
	}
}

var varRef = regexp.MustCompile(`\$\{(\w+)\}`)

// expand replaces ${name} with a value a case before saved, escaped for a
// JSON string when inJSON; a name never saved is an error.
func expand(s string, vars map[string]string, inJSON bool) (string, error) {
	var missing string

	out := varRef.ReplaceAllStringFunc(s, func(m string) string {
		v, ok := vars[m[2:len(m)-1]]
		if !ok {
			missing = m

			return m
		}

		if inJSON {
			b, _ := json.Marshal(v)

			return string(b[1 : len(b)-1])
		}

		return v
	})

	if missing != "" {
		return "", fmt.Errorf("%s was never saved", missing)
	}

	return out, nil
}

// merge sets the fields a case gives over the ones before; they stay for
// the cases after it.
func merge(a, over Auth) Auth {
	for _, f := range []struct{ to, from *string }{
		{&a.Username, &over.Username}, {&a.Password, &over.Password}, {&a.APIKey, &over.APIKey}, {&a.Bearer, &over.Bearer},
	} {
		if *f.from != "" {
			*f.to = *f.from
		}
	}

	return a
}

// expandAuth fills ${name} in the credentials; one not saved yet stays as
// it is, which the server then refuses.
func expandAuth(a Auth, vars map[string]string) Auth {
	for _, f := range []*string{&a.Username, &a.Password, &a.APIKey, &a.Bearer} {
		if v, err := expand(*f, vars, false); err == nil {
			*f = v
		}
	}

	return a
}

// save keeps values of the result for later cases: name -> a dotted path
// into it ("items.0.id"), the whole result for "".
func save(result []byte, paths map[string]string, vars map[string]string) {
	for name, path := range paths {
		var v any
		if json.Unmarshal(result, &v) != nil {
			continue
		}

		for _, key := range strings.Split(path, ".") {
			if key == "" {
				continue
			}

			switch x := v.(type) {
			case map[string]any:
				v = x[key]
			case []any:
				i, _ := strconv.Atoi(key)
				if i < len(x) {
					v = x[i]
				}
			}
		}

		if s, ok := v.(string); ok {
			vars[name] = s
		} else {
			b, _ := json.Marshal(v)
			vars[name] = string(b)
		}
	}
}

// numbered is key, or key2, key3 when taken: a generator numbers the second
// parameter of one identifier the same way.
func numbered(taken map[string]string, key string) string {
	if _, ok := taken[key]; !ok {
		return key
	}

	for n := 2; ; n++ {
		if k := key + strconv.Itoa(n); taken[k] == "" {
			return k
		}
	}
}

// norm drops all but letters and digits and lowers them, so a spec name
// and the identifier a generator makes of it compare equal.
func norm(s string) string {
	var b strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}

	return b.String()
}
