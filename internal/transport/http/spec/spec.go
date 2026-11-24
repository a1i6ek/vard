// Package spec holds the API's answers to the schemas its description
// (api/openapi) declares for them: a field renamed on one side and not the
// other would otherwise only show as something missing on a page, since both
// apps' types are generated from the description. Tests use it; nothing else
// does.
package spec

import (
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/a1i6ek/vard/api/openapi"
)

// Match fails the test when the schema `name` and the JSON that `value`
// encodes to do not have the same fields. `serverOnly` names fields the
// server sends that the schema deliberately leaves out.
func Match(t *testing.T, name string, value any, serverOnly ...string) {
	t.Helper()

	declared, err := Properties(name)
	if err != nil {
		t.Fatal(err)
	}
	sent := JSONFields(reflect.TypeOf(value))
	for _, field := range sent {
		if !slices.Contains(declared, field) && !slices.Contains(serverOnly, field) {
			t.Errorf("%s: the server sends %q, which the schema does not declare", name, field)
		}
	}
	for _, field := range declared {
		if !slices.Contains(sent, field) {
			t.Errorf("%s: the schema declares %q, which the server never sends", name, field)
		}
	}
}

// Properties are the fields schema `name` declares, in the admin document
// or else the account one, through allOf and $ref.
func Properties(name string) ([]string, error) {
	for _, root := range []string{openapi.AdminRoot, openapi.AccountRoot} {
		document, err := openapi.Bundle(root)
		if err != nil {
			return nil, err
		}
		schemas, _ := document["components"].(map[string]any)["schemas"].(map[string]any)
		if schema, ok := schemas[name]; ok {
			fields := map[string]bool{}
			collect(schema, schemas, fields, map[string]bool{})
			out := make([]string, 0, len(fields))
			for field := range fields {
				out = append(out, field)
			}
			sort.Strings(out)
			return out, nil
		}
	}
	return nil, &missing{name}
}

type missing struct{ name string }

func (m *missing) Error() string { return "spec: no schema " + m.name }

func collect(node any, schemas map[string]any, fields, seen map[string]bool) {
	schema, ok := node.(map[string]any)
	if !ok {
		return
	}
	if ref, ok := schema["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		if !seen[name] {
			seen[name] = true
			collect(schemas[name], schemas, fields, seen)
		}
		return
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for field := range properties {
			fields[field] = true
		}
	}
	for _, key := range []string{"allOf"} {
		if parts, ok := schema[key].([]any); ok {
			for _, part := range parts {
				collect(part, schemas, fields, seen)
			}
		}
	}
}
func JSONFields(typ reflect.Type) []string {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	var names []string
	for i := range typ.NumField() {
		field := typ.Field(i)
		tag, _, _ := strings.Cut(field.Tag.Get("json"), ",")

		switch {
		case tag == "-":
		case field.Anonymous && tag == "":
			names = append(names, JSONFields(field.Type)...)
		case !field.IsExported():
		case tag != "":
			names = append(names, tag)
		default:
			names = append(names, field.Name)
		}
	}

	return names
}
