package v1

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
)

func TestCSVPublishedContracts(t *testing.T) {
	data, err := os.ReadFile("../../static/docs/swagger.json")
	require.NoError(t, err)
	var spec struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct{ Schema map[string]any }
		}
		Definitions map[string]struct {
			Required   []string
			Properties map[string]struct {
				Enum   []string
				Type   string
				Format string
				Ref    string `json:"$ref"`
			}
		}
	}
	require.NoError(t, json.Unmarshal(data, &spec))
	for _, prefix := range []string{"/v1/group/csv-exports", "/v1/users/self/csv-exports"} {
		for path, methods := range map[string][]string{"": {"get", "post"}, "/{id}": {"get", "delete"}, "/{id}/download": {"get"}, "/{id}/retry": {"post"}, "/retention": {"get", "put"}} {
			for _, method := range methods {
				op, ok := spec.Paths[prefix+path][method]
				require.True(t, ok, "%s %s", method, prefix+path)
				require.NotEmpty(t, op.Responses["404"])
				if method == "delete" || path == "/{id}/retry" {
					require.Empty(t, op.Responses["202"].Schema, "202 is an empty body")
				}
				if path == "/{id}/download" {
					require.Equal(t, "file", op.Responses["200"].Schema["type"])
				}
			}
		}
	}
	var rawSpec map[string]any
	require.NoError(t, json.Unmarshal(data, &rawSpec))
	defs := rawSpec["definitions"].(map[string]any)
	captureProps := defs["services.CSVCaptureRequest"].(map[string]any)["properties"].(map[string]any)
	columnEnums := captureProps["columns"].(map[string]any)["items"].(map[string]any)["enum"].([]any)
	seenColumns := map[string]bool{}
	for _, source := range services.CSVSourceCatalogue() {
		for _, column := range source.Columns {
			seenColumns[string(column)] = true
		}
	}
	require.Len(t, columnEnums, len(seenColumns))
	for _, enum := range columnEnums {
		require.True(t, seenColumns[enum.(string)])
	}
	capture := spec.Definitions["services.CSVCaptureRequest"]
	require.ElementsMatch(t, []string{"source", "scope", "columns"}, capture.Required)
	var sources []string
	for _, s := range services.CSVSourceCatalogue() {
		sources = append(sources, string(s.Source))
	}
	require.ElementsMatch(t, sources, capture.Properties["source"].Enum)
	require.ElementsMatch(t, sources, spec.Definitions["services.CSVArtifact"].Properties["source"].Enum)
	for _, schema := range []string{"services.CSVCaptureRequest", "services.CSVArtifact"} {
		for _, secret := range []string{"blobKey", "blob_key", "capture", "failureDetail", "token", "hash", "pepper", "role", "userId"} {
			_, exists := spec.Definitions[schema].Properties[secret]
			require.False(t, exists, "%s exposes %s", schema, secret)
		}
	}
	for _, pair := range [][2]string{{"swagger.json", "swagger-2.0.json"}, {"swagger.yaml", "swagger-2.0.yaml"}, {"openapi-3.json", "openapi-3.0.json"}, {"openapi-3.yaml", "openapi-3.0.yaml"}} {
		server, err := os.ReadFile("../../static/docs/" + pair[0])
		require.NoError(t, err)
		published, err := os.ReadFile("../../../../../docs/public/api/" + pair[1])
		require.NoError(t, err)
		require.Equal(t, string(server), string(published))
	}
	var openapi map[string]any
	data, err = os.ReadFile("../../static/docs/openapi-3.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &openapi))
	for _, prefix := range []string{"/v1/group/csv-exports", "/v1/users/self/csv-exports"} {
		responses := openapi["paths"].(map[string]any)[prefix+"/{id}/download"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)
		require.Contains(t, responses["200"].(map[string]any)["content"], "text/csv")
		require.Contains(t, responses["404"].(map[string]any)["content"], "application/json")
	}
}
