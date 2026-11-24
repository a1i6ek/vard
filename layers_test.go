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

// The Go code follows domain-driven design layering, and this test is the
// rule that keeps it that way. Each package sits in exactly one layer and
// may import only what its layer allows — never a layer above it:
//
//   - platform is the shared kernel: primitives that know nothing of this
//     server (crypto), plus brand and config.
//   - domain is the centre: model, the entities, value objects and
//     aggregates, knowing no database, HTTP or use case.
//   - infrastructure adapts the outside world (store, database, cache,
//     mail, passkey, signing) to what the domain needs.
//   - application is the provider's use cases (ADR 0003), layered of their
//     own: what every part shares (authreq) below sign-in, sign-in below
//     the protocol, and OpenID Connect and federation on top, neither
//     importing the other.
//   - transport is the HTTP surface (was internal/api): only it answers
//     requests.
//   - app is the composition root: the one place every layer is wired
//     together, and cmd only parses its flags and calls it.
//
// Migrations, i18n and the OpenAPI spec stand outside the layers; the
// federation client (outbound) is an infrastructure tool of its own.
var layerOf = map[string]string{
	"internal/platform":                           "platform",
	"internal/platform/crypto":                    "platform",
	"internal/platform/brand":                     "brand",
	"internal/platform/config":                    "config",
	"internal/domain":                             "domain",
	"internal/domain/model":                       "domain",
	"internal/infrastructure":                     "infrastructure",
	"internal/infrastructure/cache":               "infrastructure",
	"internal/infrastructure/database":            "infrastructure",
	"internal/infrastructure/mail":                "infrastructure",
	"internal/infrastructure/passkey":             "infrastructure",
	"internal/infrastructure/store":               "infrastructure",
	"internal/infrastructure/database/migrations": "migrations",
	"i18n":                             "i18n",
	"api/openapi":                      "spec",
	"internal/infrastructure/outbound": "outbound",
	"internal/infrastructure/signing":  "signing",
	"internal/application":             "application",
	"internal/application/authreq":     "authreq",
	"internal/application/signin":      "signin",
	"internal/application/oauth":       "oauth",
	"internal/application/oidc":        "oidc",
	"internal/application/federation":  "federation",
	"internal/application/admin":       "admin",
	"internal/transport":               "transport",
	"internal/transport/http":          "transport",
	"internal/app":                     "app",
	"cmd":                              "cmd",
}

var layers = map[string][]string{
	"platform":       {"platform"},
	"brand":          {},
	"config":         {"brand"},
	"domain":         {"platform", "brand", "config", "domain"},
	"migrations":     {"platform", "brand", "config", "domain", "migrations"},
	"i18n":           {"i18n"},
	"spec":           {"spec"},
	"infrastructure": {"platform", "brand", "config", "domain", "migrations", "i18n", "infrastructure"},
	"outbound":       {"platform"},
	"signing":        {"platform", "brand", "config", "domain", "i18n", "infrastructure"},
	"application":    {"platform", "brand", "config", "domain", "i18n", "infrastructure", "outbound", "authreq", "signin", "oauth", "oidc", "federation", "admin"},
	"authreq":        {"platform", "brand", "config", "domain", "i18n", "infrastructure"},
	"signin":         {"platform", "brand", "config", "domain", "i18n", "infrastructure", "outbound", "authreq"},
	"oauth":          {"platform", "brand", "config", "domain", "i18n", "infrastructure", "outbound", "signing", "authreq", "signin"},
	"oidc":           {"platform", "brand", "config", "domain", "i18n", "infrastructure", "outbound", "signing", "authreq", "signin", "oauth"},
	"federation":     {"platform", "brand", "config", "domain", "i18n", "infrastructure", "outbound", "authreq", "signin", "oauth"},
	"admin":          {"platform", "brand", "config", "domain", "i18n", "infrastructure"},
	"transport":      {"platform", "brand", "config", "domain", "i18n", "spec", "infrastructure", "outbound", "signing", "authreq", "signin", "oauth", "oidc", "federation", "admin", "transport"},
	"app":            {"platform", "brand", "config", "domain", "migrations", "i18n", "infrastructure", "outbound", "signing", "authreq", "signin", "oauth", "oidc", "federation", "admin", "transport"},
	"cmd":            {"brand", "config", "domain", "migrations", "app"},
}

var walked = []string{"internal", "cmd", "api"}

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
