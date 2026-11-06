package apidoc

import (
	"encoding/json"
	"go/ast"
	"go/constant"
	"go/types"
	"slices"
)

// responseLiterals describes fixed envelope keys written as map literals, without changing
// their runtime encoding. Unknown/dynamic keys deliberately fall back to the map's Go type.
func responseLiterals(info *types.Info, s *schemas, values []ast.Expr) map[string]any {
	var out map[string]any
	for _, value := range values {
		schema := literalObject(info, s, value)
		if schema == nil {
			return nil
		}
		if out == nil {
			out = schema
		} else {
			out = mergeSchema(out, schema)
		}
	}
	return out
}

// Resolve helpers only when their returns are all fixed map literals. No dynamic keys or
// interface-valued callback payloads are guessed (for example the ffprobe status extension).
func responseLiteralsWithHelpers(info *types.Info, s *schemas, values []ast.Expr, funcs map[types.Object]*ast.FuncDecl) map[string]any {
	expanded := []ast.Expr{}
	for _, value := range values {
		call, ok := unparen(value).(*ast.CallExpr)
		if !ok {
			expanded = append(expanded, value)
			continue
		}
		var name *ast.Ident
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			name = fn
		case *ast.SelectorExpr:
			name = fn.Sel
		}
		if name == nil {
			return nil
		}
		decl := funcs[info.Uses[name]]
		if decl == nil {
			return nil
		}
		returns := []ast.Expr{}
		invalid := false
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			if _, ok := node.(*ast.FuncLit); ok {
				return false
			}
			if result, ok := node.(*ast.ReturnStmt); ok {
				if len(result.Results) != 1 {
					invalid = true
				} else {
					returns = append(returns, result.Results[0])
				}
			}
			return true
		})
		if invalid || len(returns) == 0 {
			return nil
		}
		expanded = append(expanded, returns...)
	}
	return responseLiterals(info, s, expanded)
}

func literalObject(info *types.Info, s *schemas, value ast.Expr) map[string]any {
	value = unparen(value)
	lit, ok := value.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	typ := info.TypeOf(lit)
	if typ == nil {
		return nil
	}
	m, ok := typ.Underlying().(*types.Map)
	if !ok || m.Key().String() != "string" {
		return nil
	}
	props := map[string]any{}
	required := []string{}
	for _, element := range lit.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return nil
		}
		key := info.Types[pair.Key].Value
		if key == nil || key.Kind() != constant.String {
			return nil
		}
		name := constant.StringVal(key)
		schema := literalObject(info, s, pair.Value)
		if schema == nil {
			t := info.TypeOf(pair.Value)
			if id, ok := pair.Value.(*ast.Ident); ok && id.Name == "nil" {
				schema = map[string]any{"type": "null"}
			} else {
				schema = s.of(t)
				if t != nil {
					switch t.Underlying().(type) {
					case *types.Pointer, *types.Slice, *types.Map:
						schema = mergeSchema(schema, map[string]any{"type": "null"})
					}
				}
			}
		}
		props[name] = schema
		required = append(required, name)
	}
	slices.Sort(required)
	return map[string]any{"type": "object", "properties": props, "required": required}
}
func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}
func schemaEqual(a, b map[string]any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// Merge alternative control-flow branches. Fixed envelopes merge fields, requiring only keys
// present in every branch; scalar/null and nullable collection alternatives stay explicit.
func mergeSchema(a, b map[string]any) map[string]any {
	if schemaEqual(a, b) {
		return a
	}
	if a["type"] == "object" && b["type"] == "object" && a["properties"] != nil && b["properties"] != nil {
		ap := a["properties"].(map[string]any)
		bp := b["properties"].(map[string]any)
		props := map[string]any{}
		for key, value := range ap {
			props[key] = value
		}
		for key, value := range bp {
			if old, ok := props[key]; ok {
				props[key] = mergeSchema(old.(map[string]any), value.(map[string]any))
			} else {
				props[key] = value
			}
		}
		ar, _ := a["required"].([]string)
		br, _ := b["required"].([]string)
		required := []string{}
		for _, key := range ar {
			if slices.Contains(br, key) {
				required = append(required, key)
			}
		}
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	alternatives := []any{}
	add := func(v map[string]any) {
		list, ok := v["anyOf"].([]any)
		if !ok {
			list = []any{v}
		}
		for _, candidate := range list {
			found := false
			for _, existing := range alternatives {
				if schemaEqual(existing.(map[string]any), candidate.(map[string]any)) {
					found = true
					break
				}
			}
			if !found {
				alternatives = append(alternatives, candidate)
			}
		}
	}
	add(a)
	add(b)
	return map[string]any{"anyOf": alternatives}
}
