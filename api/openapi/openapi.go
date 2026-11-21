// Package openapi is the HTTP API's description (OpenAPI 3.1), the source of
// truth for both apps' types and for the tests that hold the server to it.
// It is kept split by tag (account/*.yaml, admin/*.yaml), with the problem
// details both share in common.yaml, and bundled into one document here.
package openapi

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

//go:embed *.yaml account/*.yaml admin/*.yaml
var files embed.FS

// The two documents, by their root file.
const (
	AccountRoot = "account.yaml"
	AdminRoot   = "admin.yaml"
)

var (
	bundled   = map[string][]byte{}
	bundledMu sync.Mutex
)

// JSON is the document rooted at root, bundled into one: every path and
// schema inlined or gathered under components, so it reads without the
// files it was split into.
func JSON(root string) ([]byte, error) {
	bundledMu.Lock()
	defer bundledMu.Unlock()

	if document, ok := bundled[root]; ok {
		return document, nil
	}
	document, err := Bundle(root)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	bundled[root] = encoded
	return encoded, nil
}

// Bundle reads the document rooted at root into one tree.
func Bundle(root string) (map[string]any, error) {
	b := &bundler{loaded: map[string]any{}, schemas: map[string]any{}, responses: map[string]any{}, parameters: map[string]any{}}
	document, err := b.load(root)
	if err != nil {
		return nil, err
	}
	top, ok := document.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("openapi: %s is not a document", root)
	}

	paths, _ := top["paths"].(map[string]any)
	for key, item := range paths {
		resolved, err := b.resolve(item, root)
		if err != nil {
			return nil, fmt.Errorf("openapi: %s: %w", key, err)
		}
		paths[key] = resolved
	}

	components, _ := top["components"].(map[string]any)
	if components == nil {
		components = map[string]any{}
		top["components"] = components
	}
	listed, _ := components["schemas"].(map[string]any)
	for name, entry := range listed {
		if _, err := b.resolve(entry, root); err != nil {
			return nil, fmt.Errorf("openapi: schema %s: %w", name, err)
		}
	}
	components["schemas"] = b.schemas
	if len(b.responses) > 0 {
		components["responses"] = b.responses
	}
	if len(b.parameters) > 0 {
		components["parameters"] = b.parameters
	}
	return top, nil
}

type bundler struct {
	loaded     map[string]any
	schemas    map[string]any
	responses  map[string]any
	parameters map[string]any
}

func (b *bundler) load(name string) (any, error) {
	if document, ok := b.loaded[name]; ok {
		return document, nil
	}
	data, err := files.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	b.loaded[name] = document
	return document, nil
}

// resolve walks node, read from file, turning every $ref into the document's
// own: a schema file's entry becomes a component, a common.yaml response a
// component response, and anything else (a path item) is inlined.
func (b *bundler) resolve(node any, file string) (any, error) {
	switch value := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(value))
		// OpenAPI 3.1 lets a $ref stand beside a description or the like:
		// the reference is followed, and what is beside it kept.
		if ref, ok := value["$ref"].(string); ok {
			followed, err := b.follow(ref, file)
			if err != nil || len(value) == 1 {
				return followed, err
			}
			for key, child := range followed.(map[string]any) {
				out[key] = child
			}
		}
		for key, child := range value {
			if key == "$ref" {
				continue
			}
			resolved, err := b.resolve(child, file)
			if err != nil {
				return nil, err
			}
			out[key] = resolved
		}
		return out, nil
	case []any:
		out := make([]any, len(value))
		for i, child := range value {
			resolved, err := b.resolve(child, file)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	default:
		return node, nil
	}
}

func (b *bundler) follow(ref, file string) (any, error) {
	target, pointer, _ := strings.Cut(ref, "#")
	if target == "" {
		target = file
	} else {
		target = path.Clean(path.Join(path.Dir(file), target))
	}

	name := pointerName(pointer)
	switch {
	case target == "common.yaml" && strings.HasSuffix(name, "Parameter"):
		return b.component(b.parameters, "parameters", target, pointer, name)
	case strings.HasSuffix(target, "schemas.yaml") || target == "common.yaml" && !strings.HasSuffix(name, "Response"):
		if _, done := b.schemas[name]; !done {
			b.schemas[name] = nil // a schema that refers to itself
			schema, err := b.at(target, pointer)
			if err != nil {
				return nil, err
			}
			if b.schemas[name], err = b.resolve(schema, target); err != nil {
				return nil, err
			}
		}
		return map[string]any{"$ref": "#/components/schemas/" + name}, nil
	case target == "common.yaml":
		return b.component(b.responses, "responses", target, pointer, name)
	case target == file && file == AccountRoot || target == file && file == AdminRoot:
		return map[string]any{"$ref": ref}, nil
	default:
		node, err := b.at(target, pointer)
		if err != nil {
			return nil, err
		}
		return b.resolve(node, target)
	}
}

// component gathers what pointer names in target under components.<kind>,
// once, and refers to it there.
func (b *bundler) component(into map[string]any, kind, target, pointer, name string) (any, error) {
	if _, done := into[name]; !done {
		node, err := b.at(target, pointer)
		if err != nil {
			return nil, err
		}
		if into[name], err = b.resolve(node, target); err != nil {
			return nil, err
		}
	}
	return map[string]any{"$ref": "#/components/" + kind + "/" + name}, nil
}

// at is the node pointer names in file (RFC 6901).
func (b *bundler) at(file, pointer string) (any, error) {
	node, err := b.load(file)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if part == "" {
			continue
		}
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		object, ok := node.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s#%s: not an object at %q", file, pointer, part)
		}
		if node, ok = object[part]; !ok {
			return nil, fmt.Errorf("%s#%s: no %q", file, pointer, part)
		}
	}
	return node, nil
}

func pointerName(pointer string) string {
	parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	return strings.ReplaceAll(strings.ReplaceAll(parts[len(parts)-1], "~1", "/"), "~0", "~")
}
