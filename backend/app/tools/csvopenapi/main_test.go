package main

import "testing"

func TestDownloadResponseMediaTypes(t *testing.T) {
	success := map[string]any{"content": map[string]any{"text/csv": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}}
	failure := map[string]any{"content": map[string]any{"text/csv": map[string]any{"schema": map[string]any{"type": "object"}}}}
	spec := map[string]any{"paths": map[string]any{"/v1/users/self/csv-exports/{id}/download": map[string]any{"get": map[string]any{"responses": map[string]any{"200": success, "404": failure}}}}}
	refine(spec)
	if success["content"].(map[string]any)["text/csv"] == nil || success["headers"] == nil {
		t.Fatal("lost binary success/headers")
	}
	if failure["content"].(map[string]any)["application/json"] == nil || failure["content"].(map[string]any)["text/csv"] != nil {
		t.Fatal("errors must be JSON")
	}
}
