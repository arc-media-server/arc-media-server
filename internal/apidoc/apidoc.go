// Package apidoc writes docs/API.md, the reference for Arc's API, from the code: every route
// registered in internal/api/api.go, who may call it, and the handler's doc comment (or the
// README's API table where a handler has none). A test keeps the file up to date.
package apidoc

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type route struct {
	method, path, access, handler string
}

// routes inspects registrations structurally. Unknown wrappers fail generation rather than
// silently removing endpoints from the public API contract.
func routes(apiDir string) ([]route, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(apiDir, "api.go"), nil, 0)
	if err != nil {
		return nil, err
	}
	var out []route
	var routeErr error
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandleFunc" || len(call.Args) != 2 {
			return true
		}
		receiver, ok := sel.X.(*ast.Ident)
		if !ok || receiver.Name != "mux" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			routeErr = fmt.Errorf("API route pattern must be a literal")
			return false
		}
		pattern, err := strconv.Unquote(literal.Value)
		if err != nil {
			routeErr = err
			return false
		}
		method, path, found := strings.Cut(pattern, " ")
		if !found || !strings.HasPrefix(path, "/api/") {
			return true
		} // catch-all and web UI
		handler, access, err := routeHandler(call.Args[1], "anyone")
		if err != nil {
			routeErr = fmt.Errorf("%s: %w", pattern, err)
			return false
		}
		out = append(out, route{method: method, path: path, access: access, handler: handler})
		return false
	})
	return out, routeErr
}

func routeHandler(expr ast.Expr, access string) (string, string, error) {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		if receiver, ok := e.X.(*ast.Ident); ok && receiver.Name == "s" {
			return e.Sel.Name, access, nil
		}
	case *ast.CallExpr:
		if len(e.Args) != 1 {
			break
		}
		if wrapper, ok := e.Fun.(*ast.Ident); ok && (wrapper.Name == "user" || wrapper.Name == "admin") {
			if wrapper.Name == "user" {
				access = "profile"
			} else {
				access = "admin"
			}
			return routeHandler(e.Args[0], access)
		}
		if wrapper, ok := e.Fun.(*ast.SelectorExpr); ok && wrapper.Sel.Name == "logSettingChanges" {
			if receiver, ok := wrapper.X.(*ast.Ident); ok && receiver.Name == "s" {
				return routeHandler(e.Args[0], access)
			}
		}
	}
	return "", "", fmt.Errorf("unsupported API handler/wrapper; teach the route extractor before adding it")
}

// handlerDocs maps each Server method to its doc comment.
func handlerDocs(apiDir string) (map[string]string, error) {
	fset := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join(apiDir, "*.go"))
	if err != nil {
		return nil, err
	}
	docs := map[string]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Doc == nil {
				continue
			}
			docs[fn.Name.Name] = strings.Join(strings.Fields(fn.Doc.Text()), " ")
		}
	}
	return docs, nil
}

// methodsRe matches "GET" or "GET/PUT/DELETE".
var methodsRe = regexp.MustCompile(`^[A-Z]+(/[A-Z]+)*$`)

var readmeRowRe = regexp.MustCompile("^\\| ([A-Z /]+) \\| `([^`]+)` \\| (.*) \\|$")

// readmeRows maps "METHOD path" to the README API table's description.
func readmeRows(readme string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(readme)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		m := readmeRowRe.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		methods := strings.Split(m[1], " / ")
		descs := strings.Split(m[3], " / ")
		for i, method := range methods {
			d := m[3]
			if len(descs) == len(methods) {
				d = descs[i]
			}
			path, _, _ := strings.Cut(m[2], "?")
			out[strings.TrimSpace(method)+" "+path] = strings.TrimSpace(d)
		}
	}
	return out
}

// describe turns a doc comment like "castLink: POST /api/cast {"url"} turns ..." into the part
// after the route.
func describe(doc string, r route) string {
	if i := strings.Index(doc, ": "); i > 0 && !strings.Contains(doc[:i], " ") {
		doc = doc[i+2:]
	}
	doc = strings.TrimPrefix(doc, r.handler+" ")
	for _, prefix := range []string{r.method + " " + r.path, r.method + "/"} {
		if strings.HasPrefix(doc, prefix) {
			doc = strings.TrimSpace(doc[len(prefix):])
		}
	}
	// Doc comments for handlers serving several routes name them all ("GET/PUT/DELETE ...").
	if f := strings.Fields(doc); len(f) > 1 && strings.HasPrefix(f[0], "/api/") {
		doc = strings.TrimSpace(strings.TrimPrefix(doc, f[0]))
	}
	// Another method and path first (a handler serving several routes).
	if f := strings.Fields(doc); len(f) > 1 && methodsRe.MatchString(f[0]) && strings.HasPrefix(f[1], "/api/") {
		doc = strings.Join(f[2:], " ")
	}
	for _, who := range []string{"(anyone) ", "(admin) ", "(admin)", "(anyone)"} {
		doc = strings.ReplaceAll(doc, who, "")
	}
	doc = strings.TrimLeft(doc, ". ")
	if doc != "" {
		doc = strings.ToUpper(doc[:1]) + doc[1:]
	}
	return doc
}

// Generate returns docs/API.md for the repository at root.
func Generate(root string) (string, error) {
	apiDir := filepath.Join(root, "internal", "api")
	rs, err := routes(apiDir)
	if err != nil {
		return "", err
	}
	docs, err := handlerDocs(apiDir)
	if err != nil {
		return "", err
	}
	readme := readmeRows(filepath.Join(root, "README.md"))
	groups := map[string][]route{}
	for _, r := range rs {
		seg := strings.Split(strings.TrimPrefix(r.path, "/api/"), "/")[0]
		groups[seg] = append(groups[seg], r)
	}
	names := make([]string, 0, len(groups))
	for g := range groups {
		names = append(names, g)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString(intro)
	fmt.Fprintf(&b, "\n## Endpoints\n\n%d endpoints, grouped by the first part of their path. **Who** is who may call it: `anyone`, any signed-in `profile`, or the `admin` profile.\n", len(rs))
	for _, g := range names {
		fmt.Fprintf(&b, "\n### %s\n\n| Method | Path | Who | What it does |\n|---|---|---|---|\n", g)
		for _, r := range groups[g] {
			d := describe(docs[r.handler], r)
			if d == "" {
				d = readme[r.method+" "+r.path]
			}
			d = strings.ReplaceAll(d, "|", "\\|")
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", r.method, r.path, r.access, d)
		}
	}
	return b.String(), nil
}

const intro = `# Arc API

<!-- Generated by ` + "`go run ./tools/apidoc`" + ` from internal/api. Don't edit by hand. -->

Arc's web app uses this API, and so can any other app. Most requests and responses are JSON;
media and live events use their documented content types. Start with [Client integration guide](CLIENT-INTEGRATION.md)
for pagination, capabilities, playback and executable examples. The server's /docs/ page is an
interactive viewer of the generated OpenAPI contract.

## Versions

Apps should call **` + "`/api/v1/...`" + `**, the stable API: every endpoint below is there with ` + "`/api/`" + `
replaced by ` + "`/api/v1/`" + `. Endpoints won't be removed or changed incompatibly within v1 (new
fields and endpoints may appear, so ignore what you don't know). The unversioned ` + "`/api/...`" + ` paths
are the web app's and change with it. ` + "`GET /api/v1/health`" + ` answers with ` + "`api_version`" + `.

## Signing in

The web app uses a cookie. Apps send a token instead: ` + "`Authorization: Bearer <token>`" + `.

- **With the password**: ` + "`POST /api/v1/auth/login {\"password\", \"device\"}`" + ` → ` + "`{\"token\"}`" + `. ` + "`device`" + ` is
  the app's name for itself (e.g. "Living Room Apple TV"), shown in Settings > Devices, where the
  admin can sign it out. Then pick a profile: ` + "`GET /api/v1/auth/status`" + ` lists them, and
  ` + "`POST /api/v1/auth/profile {\"id\", \"pin\"}`" + ` uses one.
- **A tool**: Settings → Access & devices → Devices → New token for a tool (admin only).
  Arc issues a separate named device token for each tool, shown once. Send the same Authorization
  Bearer header; revoke it in Devices. Tool tokens use the existing device-session storage and checks.
- **A friend**: ` + "`POST /api/v1/auth/friend {\"name\", \"password\", \"device\"}`" + ` → ` + "`{\"token\"}`" + `, already
  using their profile.
- **On a TV, with a code**: ` + "`POST /api/v1/auth/link {\"device\"}`" + ` →
  ` + "`{\"code\", \"poll\", \"expires_in\"}`" + `. Show the code and ask the person to enter it in Arc (the profile
  menu's *Link a Device*, or ` + "`/link`" + `). Meanwhile ask ` + "`GET /api/v1/auth/link/{poll}`" + ` every few
  seconds: 202 while waiting, then 200 with ` + "`{\"token\"}`" + ` once; 404 when the code has run out (10
  minutes).
- **At home**, when the admin allows it, profiles can be picked without signing in
  (` + "`bypass`" + ` in ` + "`/auth/status`" + `); the answer then sets a cookie.

## Errors

Failures answer with an HTTP error status and ` + "`{\"error\": \"what went wrong, in plain words\"}`" + `,
sometimes with ` + "`needs`" + ` (e.g. ` + "`\"pin\"`" + `, ` + "`\"password\"`" + `, ` + "`\"stopped\"`" + `) saying what to do next. 401 means
sign in (or choose a profile), 403 not allowed. Common JSON errors additionally include a stable
HTTP-category code and request_id, matching the server-issued X-Request-ID response header.
Older servers and plain-text transport errors may omit these fields; check HTTP status first.

## Playing

` + "`POST /api/v1/items/{id}/playback`" + ` with what the player can play (` + "`caps`" + `) answers with a URL to play:
the file as it is, or an HLS stream Arc converts (keep it alive with ` + "`POST /transcode/{sid}/ping`" + `).
Send progress with ` + "`PUT /items/{id}/progress`" + `. Players that can't send the Authorization header
(e.g. a cast receiver) can turn that URL into a link with a token in it: ` + "`POST /api/v1/cast {\"url\"}`" + `.
`

// Write regenerates docs/API.md.
func Write(root string) error {
	doc, err := Generate(root)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "docs", "API.md"), []byte(doc), 0o644)
}
