package api

import (
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/api/openapi"
	"github.com/a1i6ek/vard/internal/transport/http/respond"
)

// The description is the source of truth for both apps' types, so it has
// every route the servers answer, and nothing they do not: each method on
// each path, the public server's in account.yaml, the admin server's in
// admin.yaml. Only the orchestrator's probes are left out.
func TestTheDescriptionHasEveryRouteAndNoOther(t *testing.T) {
	tests := []struct {
		name   string
		engine *gin.Engine
		root   string
	}{
		{"the public server", testEngine(), openapi.AccountRoot},
		{"the admin server", testAdminEngine(), openapi.AdminRoot},
	}

	parameter := regexp.MustCompile(`:(\w+)`)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := openapi.Bundle(tt.root)
			if err != nil {
				t.Fatal(err)
			}
			described := map[string]bool{}
			for path, item := range document["paths"].(map[string]any) {
				for method := range item.(map[string]any) {
					if slices.Contains([]string{"get", "post", "put", "patch", "delete"}, method) {
						described[strings.ToUpper(method)+" "+path] = true
					}
				}
			}

			served := map[string]bool{}
			for _, route := range tt.engine.Routes() {
				if route.Path == "/healthz" || route.Path == "/readyz" {
					continue
				}
				served[route.Method+" "+parameter.ReplaceAllString(route.Path, "{$1}")] = true
			}

			for _, key := range sortedKeys(served) {
				if !described[key] {
					t.Errorf("%s is served but not in %s", key, tt.root)
				}
			}
			for _, key := range sortedKeys(described) {
				if !served[key] {
					t.Errorf("%s is in %s but nothing serves it", key, tt.root)
				}
			}
		})
	}
}

// Every problem the API can answer with is named in the description's
// ProblemCode, and nothing else is, so an app's types know every code.
func TestTheDescriptionNamesEveryProblem(t *testing.T) {
	document, err := openapi.Bundle(openapi.AdminRoot)
	if err != nil {
		t.Fatal(err)
	}
	schema := document["components"].(map[string]any)["schemas"].(map[string]any)["ProblemCode"].(map[string]any)
	var listed []string
	for _, code := range schema["enum"].([]any) {
		listed = append(listed, code.(string))
	}

	var defined []string
	for _, problem := range respond.Problems() {
		defined = append(defined, problem.Code)
		if !slices.Contains(listed, problem.Code) {
			t.Errorf("%s is a problem the description's ProblemCode does not list", problem.Code)
		}
	}
	for _, code := range listed {
		if !slices.Contains(defined, code) {
			t.Errorf("ProblemCode lists %s, which no problem is", code)
		}
	}
}

// Both descriptions are served, through the apps' own paths, as JSON.
func TestTheDescriptionsAreServed(t *testing.T) {
	for _, tt := range []struct {
		engine *gin.Engine
		path   string
	}{
		{testEngine(), "/api/v1/account/openapi.json"},
		{testAdminEngine(), "/api/v1/admin/openapi.json"},
	} {
		w := do(tt.engine, http.MethodGet, tt.path, nil)
		if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") ||
			!strings.Contains(w.Body.String(), `"openapi":"3.1.0"`) {
			t.Errorf("%s = %d %s, want the bundled description", tt.path, w.Code, w.Header().Get("Content-Type"))
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
