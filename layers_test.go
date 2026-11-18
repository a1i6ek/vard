package layers_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The Go code is under internal/: building blocks that know nothing of this
// server (internal/crypto), the server itself, and HTTP (internal/api). Each
// package is in a layer, allowed what is below it and nothing above: the
// model knows no database, the infrastructure no use case, and only api
// answers requests.
var layerOf = map[string]string{
	"internal/crypto":              "tools",
	"internal/brand":               "brand",
	"internal/config":              "config",
	"internal/model":               "model",
	"internal/cache":               "infrastructure",
	"internal/database":            "infrastructure",
	"internal/mail":                "infrastructure",
	"internal/passkey":             "infrastructure",
	"internal/store":               "infrastructure",
	"internal/database/migrations": "migrations",
	"i18n":                         "i18n",
	"internal/admin":               "service",
	"internal/oidc":                "service",
	"internal/api":                 "api",
}

var layers = map[string][]string{
	"tools":          {"tools"},
	"brand":          {},
	"config":         {"brand"},
	"model":          {"tools", "brand", "config", "model"},
	"migrations":     {"tools", "brand", "config", "model", "migrations"},
	"i18n":           {"i18n"},
	"infrastructure": {"tools", "brand", "config", "model", "migrations", "i18n", "infrastructure"},
	"service":        {"tools", "brand", "config", "model", "i18n", "infrastructure", "service"},
	"api":            {"tools", "brand", "config", "model", "i18n", "infrastructure", "service", "api"},
}

var walked = []string{"internal"}

const module = "github.com/a1i6ek/vard/"

func TestEachLayerImportsOnlyWhatIsBelowIt(t *testing.T) {
	for _, root := range walked {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			path = filepath.ToSlash(path)
			layer, known := layerFor(path)
			if !known {
				t.Errorf("%s is in no layer; add its folder to layerOf", path)
				return nil
			}
			allowed := layers[layer]

			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, spec := range file.Imports {
				imported, _ := strconv.Unquote(spec.Path.Value)
				inside, ok := strings.CutPrefix(imported, module)
				if !ok {
					continue
				}
				to, known := layerFor(inside + "/")
				if !known {
					t.Errorf("%s imports %s, which is in no layer; add its folder to layerOf", path, imported)
					continue
				}
				if !slices.Contains(allowed, to) {
					t.Errorf("%s (%s) imports %s (%s), which is above it", path, layer, imported, to)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// layerFor is the layer of the longest folder in layerOf that holds path.
func layerFor(path string) (string, bool) {
	best, layer := "", ""
	for dir, l := range layerOf {
		if strings.HasPrefix(path, dir+"/") && len(dir) > len(best) {
			best, layer = dir, l
		}
	}
	return layer, best != ""
}
