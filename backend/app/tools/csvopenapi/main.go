// csvopenapi refines MIME types that Swagger 2 cannot express per response.
package main

import (
	"encoding/json"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func refine(spec map[string]any) {
	paths := spec["paths"].(map[string]any)
	for path, raw := range paths {
		if !strings.Contains(path, "/csv-exports") {
			continue
		}
		for _, op := range raw.(map[string]any) {
			operation, ok := op.(map[string]any)
			if !ok {
				continue
			}
			responses, ok := operation["responses"].(map[string]any)
			if !ok {
				continue
			}
			for status, rawResponse := range responses {
				response := rawResponse.(map[string]any)
				content, ok := response["content"].(map[string]any)
				if ok && strings.HasSuffix(path, "/download") && status != "200" {
					content["application/json"] = content["text/csv"]
					delete(content, "text/csv")
				}
				if status == "200" && strings.HasSuffix(path, "/download") {
					response["headers"] = map[string]any{
						"Content-Disposition":    map[string]any{"description": "Attachment filename; never a blob URL", "schema": map[string]any{"type": "string"}},
						"Cache-Control":          map[string]any{"schema": map[string]any{"type": "string", "enum": []string{"no-store"}}},
						"X-Content-Type-Options": map[string]any{"schema": map[string]any{"type": "string", "enum": []string{"nosniff"}}},
					}
				}
			}
		}
	}
}

func main() {
	if len(os.Args) != 3 {
		panic("usage: csvopenapi openapi.json openapi.yaml")
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		panic(err)
	}
	refine(spec)
	data, err = json.MarshalIndent(spec, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[1], append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	data, err = yaml.Marshal(spec)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(os.Args[2], data, 0644); err != nil {
		panic(err)
	}
}
