package openapi

import "testing"

func TestBothDocumentsBundle(t *testing.T) {
	for _, root := range []string{AccountRoot, AdminRoot} {
		document, err := Bundle(root)
		if err != nil {
			t.Fatalf("%s: %v", root, err)
		}
		paths, _ := document["paths"].(map[string]any)
		schemas, _ := document["components"].(map[string]any)["schemas"].(map[string]any)
		t.Logf("%s: %d paths, %d schemas", root, len(paths), len(schemas))
		if len(paths) == 0 || len(schemas) == 0 {
			t.Errorf("%s bundles to nothing", root)
		}
	}
}
