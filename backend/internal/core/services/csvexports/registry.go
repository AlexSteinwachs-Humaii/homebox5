package csvexports

import (
	_ "embed"
	"encoding/json"
)

//go:embed registry.json
var registryJSON []byte

type Source struct {
	Scope      string   `json:"scope"`
	Access     string   `json:"access"`
	Columns    []string `json:"columns"`
	Filters    []string `json:"filters"`
	Boundaries []string `json:"boundaries"`
}

var registry = func() map[string]Source {
	var sources map[string]Source
	if err := json.Unmarshal(registryJSON, &sources); err != nil {
		panic(err)
	}
	return sources
}()

// Lookup returns copies so callers cannot mutate the allowlist.
func Lookup(id string) (Source, bool) {
	s, ok := registry[id]
	s.Columns = append([]string(nil), s.Columns...)
	s.Filters = append([]string(nil), s.Filters...)
	s.Boundaries = append([]string(nil), s.Boundaries...)
	return s, ok
}
