package apidoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The OpenAPI document (docs/openapi.json) is built from the same routes and doc comments as
// API.md, plus what the Go code says about each handler: the type it decodes from the request
// body, the types it answers with and their statuses, the errors it can answer with and the
// query parameters it reads. Helpers in internal/api that a handler calls are followed, so
// shared lookups (like a library from its path) contribute their errors too.

// handlerInfo is what a handler's code says about its route.
type handlerInfo struct {
	request        types.Type
	responseValues map[int][]ast.Expr
	responses      map[int]types.Type // status -> body type; nil means no JSON body
	binary         bool               // writes a file or stream rather than JSON
	upload         bool               // reads the raw request body (an uploaded file)
	errors         map[int]bool
	query          map[string]bool
}

type analyzer struct {
	info  *types.Info
	funcs map[types.Object]*ast.FuncDecl
}

// loadAPI type-checks internal/api from source, importing its dependencies from the compiler's
// export data (as go/packages does), so no extra module is needed.
func loadAPI(root string) (*analyzer, map[string]*ast.FuncDecl, error) {
	cmd := exec.Command("go", "list", "-export", "-deps", "-f", "{{.ImportPath}}\t{{.Export}}", "./internal/api")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("go list: %w", err)
	}
	exports := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path, file, ok := strings.Cut(line, "\t"); ok && file != "" {
			exports[path] = file
		}
	}
	fset := token.NewFileSet()
	dir := filepath.Join(root, "internal", "api")
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, nil, err
	}
	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, f)
	}
	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(file)
	})
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	if _, err := (&types.Config{Importer: imp}).Check("ams/internal/api", fset, files, info); err != nil {
		return nil, nil, fmt.Errorf("type-checking internal/api: %w", err)
	}
	a := &analyzer{info: info, funcs: map[types.Object]*ast.FuncDecl{}}
	methods := map[string]*ast.FuncDecl{}
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if obj := info.Defs[fn.Name]; obj != nil {
				a.funcs[obj] = fn
			}
			if fn.Recv != nil {
				methods[fn.Name.Name] = fn
			}
		}
	}
	return a, methods, nil
}

// analyze collects what a handler (and the package helpers it calls) reads and writes.
func (a *analyzer) analyze(fn *ast.FuncDecl) handlerInfo {
	info := handlerInfo{responseValues: map[int][]ast.Expr{}, responses: map[int]types.Type{}, errors: map[int]bool{}, query: map[string]bool{}}
	a.walk(fn, &info, map[*ast.FuncDecl]bool{}, 0)
	return info
}

func (a *analyzer) walk(fn *ast.FuncDecl, info *handlerInfo, seen map[*ast.FuncDecl]bool, depth int) {
	if seen[fn] || depth > 3 {
		return
	}
	seen[fn] = true
	ti := a.info
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, recv := callName(call)
		switch {
		case name == "writeJSON" && recv == "" && len(call.Args) == 3:
			if status, ok := a.status(call.Args[1]); ok {
				info.responseValues[status] = append(info.responseValues[status], call.Args[2])
				// A handler can answer an empty []any in one branch and its real type in another.
				if t, seen := info.responses[status]; !seen || t == nil || vague(t) {
					info.responses[status] = ti.TypeOf(call.Args[2])
				}
			}
		case name == "writeError" && recv == "" && len(call.Args) >= 2:
			if status, ok := a.status(call.Args[1]); ok {
				info.errors[status] = true
			}
		case name == "fail" && recv != "":
			info.errors[500] = true
		case name == "WriteHeader" && len(call.Args) == 1:
			if status, ok := a.status(call.Args[0]); ok {
				if _, set := info.responses[status]; !set {
					info.responses[status] = nil
				}
			}
		case name == "decodeWatch" && recv == "" && len(call.Args) == 3:
			// The bounded shared decoder receives the concrete request pointer at the call
			// site. Its internal EOF check decodes an empty struct, not the request schema.
			if p, ok := ti.TypeOf(call.Args[2]).(*types.Pointer); ok {
				info.request = p.Elem()
			}
		case name == "Decode" && len(call.Args) == 1 && info.request == nil && isJSONDecoder(ti.TypeOf(call.Fun.(*ast.SelectorExpr).X)):
			// Handlers decode only the request body with a JSON decoder.
			if p, ok := ti.TypeOf(call.Args[0]).(*types.Pointer); ok {
				info.request = p.Elem()
			}
		case name == "Unmarshal" && recv == "json" && len(call.Args) == 2 && info.request != nil && info.request.String() == "encoding/json.RawMessage":
			if ptr, ok := ti.TypeOf(call.Args[1]).(*types.Pointer); ok {
				if _, typed := ptr.Elem().Underlying().(*types.Struct); typed {
					info.request = ptr.Elem()
				}
			}
		case name == "ReadAll" && len(call.Args) == 1 && strings.Contains(exprString(call.Args[0]), "r.Body"):
			info.upload = true
		case name == "Get" && len(call.Args) == 1 && isURLValues(ti.TypeOf(call.Fun.(*ast.SelectorExpr).X)):
			if lit, ok := call.Args[0].(*ast.BasicLit); ok {
				if v, err := strconv.Unquote(lit.Value); err == nil {
					info.query[v] = true
				}
			}
		case name == "ServeContent" || name == "ServeFile" || name == "Copy" || name == "ServeHTTP" || (name == "WriteString" && recv == "io"):
			info.binary = true
		case name == "Write" && isResponseWriter(ti.TypeOf(call.Fun.(*ast.SelectorExpr).X)):
			info.binary = true
		}
		// Follow helpers defined in internal/api.
		var id *ast.Ident
		switch f := call.Fun.(type) {
		case *ast.Ident:
			id = f
		case *ast.SelectorExpr:
			id = f.Sel
		}
		if id != nil {
			if decl := a.funcs[ti.Uses[id]]; decl != nil {
				a.walk(decl, info, seen, depth+1)
			}
		}
		return true
	})
}

func callName(call *ast.CallExpr) (name, recv string) {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name, ""
	case *ast.SelectorExpr:
		return f.Sel.Name, exprString(f.X)
	}
	return "", ""
}

func exprString(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.CallExpr:
		s := exprString(e.Fun) + "("
		for i, arg := range e.Args {
			if i > 0 {
				s += ","
			}
			s += exprString(arg)
		}
		return s + ")"
	}
	return ""
}

// vague reports whether a type says little about its JSON: any, or a list or map of any.
func vague(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Interface:
		return true
	case *types.Slice:
		return vague(u.Elem())
	case *types.Map:
		return vague(u.Elem())
	}
	return false
}

func isJSONDecoder(t types.Type) bool {
	return t != nil && t.String() == "*encoding/json.Decoder"
}

func isResponseWriter(t types.Type) bool {
	return t != nil && t.String() == "net/http.ResponseWriter"
}

func isURLValues(t types.Type) bool {
	return t != nil && t.String() == "net/url.Values"
}

// status is a constant HTTP status, such as http.StatusOK or 409.
func (a *analyzer) status(e ast.Expr) (int, bool) {
	tv, ok := a.info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return 0, false
	}
	v, ok := constant.Int64Val(tv.Value)
	return int(v), ok
}

// schemas turns Go types into JSON Schema, sharing named types as components.
type schemas struct {
	components map[string]any
	names      map[*types.TypeName]string
	input      bool
}

func (s *schemas) of(t types.Type) map[string]any {
	if t == nil {
		return map[string]any{}
	}
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		switch obj.Pkg().Path() + "." + obj.Name() {
		case "time.Time":
			return map[string]any{"type": "string", "format": "date-time"}
		case "time.Duration":
			return map[string]any{"type": "integer", "description": "nanoseconds"}
		case "encoding/json.RawMessage":
			return map[string]any{}
		}
		if hasMethod(named, "MarshalJSON") {
			return map[string]any{}
		}
		if _, ok := named.Underlying().(*types.Struct); ok {
			name, ok := s.names[obj]
			if !ok {
				name = s.name(obj)
				if s.input {
					name = "Input" + name
				}
				s.names[obj] = name
				s.components[name] = map[string]any{} // placeholder while recursing
				s.components[name] = s.of(named.Underlying())
			}
			return map[string]any{"$ref": "#/components/schemas/" + name}
		}
		return s.of(named.Underlying())
	}
	switch t := t.(type) {
	case *types.Alias:
		return s.of(types.Unalias(t))
	case *types.Pointer:
		return s.of(t.Elem())
	case *types.Basic:
		switch {
		case t.Info()&types.IsBoolean != 0:
			return map[string]any{"type": "boolean"}
		case t.Info()&types.IsInteger != 0:
			return map[string]any{"type": "integer"}
		case t.Info()&types.IsFloat != 0:
			return map[string]any{"type": "number"}
		case t.Info()&types.IsString != 0:
			return map[string]any{"type": "string"}
		}
	case *types.Slice:
		if b, ok := t.Elem().(*types.Basic); ok && b.Kind() == types.Byte {
			return map[string]any{"type": "string", "format": "byte"}
		}
		return map[string]any{"type": "array", "items": s.of(t.Elem())}
	case *types.Array:
		return map[string]any{"type": "array", "items": s.of(t.Elem())}
	case *types.Map:
		return map[string]any{"type": "object", "additionalProperties": s.of(t.Elem())}
	case *types.Struct:
		props := map[string]any{}
		var required []string
		s.fields(t, props, &required)
		out := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			sort.Strings(required)
			out["required"] = required
		}
		return out
	}
	return map[string]any{}
}

// fields adds a struct's JSON fields, flattening embedded structs as encoding/json does.
func (s *schemas) fields(st *types.Struct, props map[string]any, required *[]string) {
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		tag := reflect.StructTag(st.Tag(i)).Get("json")
		name, opts, _ := strings.Cut(tag, ",")
		if name == "-" && opts == "" {
			continue
		}
		if f.Embedded() && name == "" {
			ft := f.Type()
			if p, ok := ft.(*types.Pointer); ok {
				ft = p.Elem()
			}
			if inner, ok := ft.Underlying().(*types.Struct); ok {
				s.fields(inner, props, required)
				continue
			}
		}
		if !f.Exported() {
			continue
		}
		if name == "" {
			name = f.Name()
		}
		props[name] = s.of(f.Type())
		inputTags := reflect.StructTag(st.Tag(i)).Get("api")
		if s.input && strings.Contains(inputTags, "deprecated") {
			props[name].(map[string]any)["deprecated"] = true
			props[name].(map[string]any)["description"] = reflect.StructTag(st.Tag(i)).Get("description")
		}
		if strings.Contains(inputTags, "nullable") {
			props[name] = map[string]any{"anyOf": []any{props[name], map[string]any{"type": "null"}}}
		}
		if description := reflect.StructTag(st.Tag(i)).Get("description"); description != "" {
			props[name].(map[string]any)["description"] = description
		}
		requiredField := !strings.Contains(opts, "omitempty") && !strings.Contains(opts, "omitzero")
		if s.input {
			_, pointer := f.Type().(*types.Pointer)
			requiredField = !pointer && requiredField && !strings.Contains(inputTags, "optional")
			if strings.Contains(inputTags, "required") {
				requiredField = true
			}
		}
		if requiredField {
			*required = append(*required, name)
		}
	}
}

// name is a type's component name: its Go name without "DTO", or with its package in front
// when another package's type already has that name (e.g. BackgroundStatus beside Status).
func (s *schemas) name(obj *types.TypeName) string {
	upper := func(v string) string { return strings.ToUpper(v[:1]) + v[1:] }
	base := strings.TrimSuffix(upper(obj.Name()), "DTO")
	taken := func(name string) bool {
		for _, other := range s.names {
			if other == name {
				return true
			}
		}
		return false
	}
	if !taken(base) {
		return base
	}
	name := upper(obj.Pkg().Name()) + base
	for n := 2; taken(name); n++ {
		name = fmt.Sprintf("%s%s%d", upper(obj.Pkg().Name()), base, n)
	}
	return name
}

// prune drops components that no operation or other used component refers to.
func prune(doc map[string]any, components map[string]any) {
	for {
		data, _ := json.Marshal(doc)
		removed := false
		for name := range components {
			ref := []byte(`"#/components/schemas/` + name + `"`)
			own, _ := json.Marshal(components[name])
			if bytes.Count(data, ref) == bytes.Count(own, ref) && name != "Error" {
				delete(components, name)
				removed = true
			}
		}
		if !removed {
			return
		}
	}
}

func hasMethod(t *types.Named, name string) bool {
	for _, typ := range []types.Type{t, types.NewPointer(t)} {
		if obj, _, _ := types.LookupFieldOrMethod(typ, true, t.Obj().Pkg(), name); obj != nil {
			if _, ok := obj.(*types.Func); ok {
				return true
			}
		}
	}
	return false
}

var pathParamRe = regexp.MustCompile(`\{(\w+)(\.\.\.)?\}`)

// statusText is the response description for a status.
var statusText = map[int]string{
	200: "OK", 201: "Created", 202: "Accepted; the work continues in the background", 204: "Done; no content",
	400: "The request isn't valid", 401: "Sign in (or choose a profile) first", 403: "Not allowed for this profile",
	404: "Not found", 409: "Conflicts with the current state (busy, already done, or not possible yet)",
	410: "Gone", 413: "Too large", 415: "Unsupported type", 422: "Can't be processed", 429: "Too many attempts; wait a moment",
	500: "Something went wrong on the server", 502: "A service Arc relies on failed", 503: "Not available right now",
}

// GenerateOpenAPI returns docs/openapi.json for the repository at root.
func GenerateOpenAPI(root string) ([]byte, error) {
	apiDir := filepath.Join(root, "internal", "api")
	rs, err := routes(apiDir)
	if err != nil {
		return nil, err
	}
	docs, err := handlerDocs(apiDir)
	if err != nil {
		return nil, err
	}
	readme := readmeRows(filepath.Join(root, "README.md"))
	a, handlers, err := loadAPI(root)
	if err != nil {
		return nil, err
	}
	sch := &schemas{components: map[string]any{}, names: map[*types.TypeName]string{}}
	sch.components["Error"] = map[string]any{
		"type": "object", "required": []string{"error"},
		"properties": map[string]any{
			"error":      map[string]any{"type": "string", "description": "What went wrong, in plain words"},
			"needs":      map[string]any{"type": "string", "description": "What to do next, e.g. pin, password or stopped"},
			"code":       map[string]any{"type": "string", "description": "Stable HTTP error category; preserve unknown future/domain codes. See docs/CLIENT-INTEGRATION.md."},
			"request_id": map[string]any{"type": "string", "description": "Server-issued process-local correlation ID; matches X-Request-ID and diagnostic logs."},
		},
	}
	inputs := &schemas{components: sch.components, names: map[*types.TypeName]string{}, input: true}
	paths := map[string]map[string]any{}
	opIDs := map[string]int{}
	var missing []string
	for _, r := range rs {
		fn := handlers[r.handler]
		if fn == nil {
			return nil, fmt.Errorf("%s %s: handler %s not found", r.method, r.path, r.handler)
		}
		info := a.analyze(fn)
		desc := describe(docs[r.handler], r)
		if desc == "" {
			desc = readme[r.method+" "+r.path]
		}
		path := strings.TrimPrefix(r.path, "/api")
		op := map[string]any{
			"tags":         []string{tagName(r.path)},
			"summary":      summary(desc, r.handler),
			"description":  desc + "\n\nWho may call it: **" + r.access + "**.",
			"x-arc-access": r.access,
		}
		id := r.handler
		if opIDs[id]++; opIDs[id] > 1 {
			id += strings.Title(strings.ToLower(r.method))
		}
		op["operationId"] = id
		if r.access == "anyone" {
			op["security"] = []any{}
		}
		var params []any
		for _, m := range pathParamRe.FindAllStringSubmatch(r.path, -1) {
			params = append(params, map[string]any{"name": m[1], "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
		}
		path = pathParamRe.ReplaceAllString(path, "{$1}")
		var query []string
		for q := range info.query {
			query = append(query, q)
		}
		sort.Strings(query)
		for _, q := range query {
			param, err := queryParameter(r.handler, q)
			if err != nil {
				return nil, err
			}
			params = append(params, param)
		}
		if len(params) > 0 {
			op["parameters"] = params
		}
		if info.request != nil {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": inputs.of(info.request)}}}
		} else if info.upload {
			op["requestBody"] = map[string]any{"required": true, "description": "The file itself as the request body",
				"content": map[string]any{"application/octet-stream": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}}
		}
		responses := map[string]any{}
		for status, t := range info.responses {
			resp := map[string]any{"description": statusText[status]}
			if t != nil {
				schema := sch.of(t)
				if mapped, ok := t.Underlying().(*types.Map); ok && mapped.Key().String() == "string" {
					if inferred := responseLiteralsWithHelpers(a.info, sch, info.responseValues[status], a.funcs); inferred != nil {
						schema = inferred
					}
				}
				resp["content"] = map[string]any{"application/json": map[string]any{"schema": schema}}
			}
			responses[strconv.Itoa(status)] = resp
		}
		if r.handler == "eventStream" || r.handler == "watchTogetherEvents" {
			responses["200"] = map[string]any{"description": "Server-Sent Events; see docs/EVENTS-API.md and docs/WATCH-TOGETHER-API.md", "content": map[string]any{"text/event-stream": map[string]any{"schema": map[string]any{"type": "string"}}}}
		}
		if info.binary && responses["200"] == nil && responses["206"] == nil {
			responses["200"] = map[string]any{"description": "The file or stream", "content": map[string]any{"application/octet-stream": map[string]any{}}}
		}
		if r.access != "anyone" {
			info.errors[401] = true
			if r.method == "POST" || r.method == "PUT" || r.method == "PATCH" || r.method == "DELETE" {
				info.errors[413] = true
			}
			if r.access == "admin" {
				info.errors[403] = true
			}
		}
		for status := range info.errors {
			if _, ok := responses[strconv.Itoa(status)]; ok {
				continue
			}
			text := statusText[status]
			if text == "" {
				text = "Error"
			}
			responses[strconv.Itoa(status)] = map[string]any{"description": text,
				"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Error"}}}}
		}
		if !hasSuccess(responses) {
			missing = append(missing, fmt.Sprintf("%s %s (%s)", r.method, r.path, r.handler))
		}
		op["responses"] = responses
		supplementOperation(r, op, sch.components)
		if paths[path] == nil {
			paths[path] = map[string]any{}
		}
		paths[path][strings.ToLower(r.method)] = op
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("no success response found in the code for:\n  %s", strings.Join(missing, "\n  "))
	}
	doc := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "Arc Media Server API",
			"version":     "1",
			"description": strings.ReplaceAll(strings.TrimSpace(strings.TrimPrefix(intro, "# Arc API")), "](CLIENT-INTEGRATION.md)", "](/docs/client-integration.html)"),
		},
		"servers":  []any{map[string]any{"url": "/api/v1", "description": "The stable API for apps"}},
		"security": []any{map[string]any{"bearerAuth": []string{}}, map[string]any{"cookieAuth": []string{}}},
		"paths":    paths,
		"components": map[string]any{
			"schemas": sch.components,
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{"type": "http", "scheme": "bearer", "description": "A device token from POST /auth/login, /auth/friend, /devices/tools (admin tools) or a TV link code. Send it in this header, never in a URL."},
				"cookieAuth": map[string]any{"type": "apiKey", "in": "cookie", "name": "ams_session", "description": "The web app's session cookie."},
			},
		},
	}
	prune(doc, sch.components)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// tagName groups a route by the first part of its path: /api/library-previews/{id} is
// "Library previews".
func tagName(path string) string {
	seg := strings.Split(strings.TrimPrefix(path, "/api/"), "/")[0]
	if seg == "openapi.json" {
		return "API description"
	}
	seg = strings.ReplaceAll(seg, "-", " ")
	return strings.ToUpper(seg[:1]) + seg[1:]
}

func hasSuccess(responses map[string]any) bool {
	for status := range responses {
		if status[0] == '2' || status[0] == '3' {
			return true
		}
	}
	return false
}

// summary is a description's first sentence, without the request shapes some descriptions
// begin with or quote inline ({"name", ...}), which read badly in a sidebar.
func summary(desc, handler string) string {
	// A leading query string ("?file=<id> lists ...") or aside ("(custom orderings only) ...").
	if strings.HasPrefix(desc, "?") {
		_, desc, _ = strings.Cut(desc, " ")
	}
	if strings.HasPrefix(desc, "(") {
		if i := strings.Index(desc, ") "); i > 0 {
			desc = desc[i+2:]
		} else {
			desc = "" // only an aside: fall back to the handler's name below
		}
	}
	if strings.HasPrefix(desc, "{") || strings.HasPrefix(desc, "[") {
		depth := 0
		for i, c := range desc {
			if c == '{' || c == '[' {
				depth++
			} else if c == '}' || c == ']' {
				if depth--; depth == 0 {
					desc = strings.TrimLeft(desc[i+1:], " :,")
					break
				}
			}
		}
	}
	for _, cut := range []string{". ", ": {", " {", " (", ", e.g"} {
		if i := strings.Index(desc, cut); i > 0 {
			desc = desc[:i]
		}
	}
	desc = strings.TrimRight(desc, " ,;:")
	// "Is the profile's own rating ..." reads better as "The profile's own rating ...".
	for _, verb := range []string{"Is ", "is ", "Are ", "are "} {
		desc = strings.TrimPrefix(desc, verb)
	}
	// Too little left: say what the handler is called ("stopActivity" -> "Stop activity").
	if len(strings.Fields(desc)) < 3 {
		desc = strings.ToLower(regexp.MustCompile(`([a-z])([A-Z])`).ReplaceAllString(handler, "$1 $2"))
	}
	if desc != "" {
		desc = strings.ToUpper(desc[:1]) + desc[1:]
	}
	desc = strings.TrimSuffix(desc, ".")
	if len(desc) > 120 {
		desc = desc[:117] + "…"
	}
	return desc
}

// WriteOpenAPI regenerates docs/openapi.json.
func WriteOpenAPI(root string) error {
	doc, err := GenerateOpenAPI(root)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "docs", "openapi.json"), doc, 0o644)
}
