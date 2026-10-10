package csvexports

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A new repeated surface must have a mapped source or a reviewed explanation;
// this keeps additions from silently disappearing from the coverage matrix.
func TestEveryRepeatedSurfaceAccountedFor(t *testing.T) {
	root := "../../../../../frontend"
	data, err := os.ReadFile(filepath.Join(root, "lib/exports/coverage-surfaces.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Path           string   `json:"path"`
		Sources        []string `json:"sources"`
		ExcludedReason string   `json:"excludedReason"`
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.Path] {
			t.Fatalf("duplicate surface %s", row.Path)
		}
		seen[row.Path] = true
		if len(row.Sources) == 0 && row.ExcludedReason == "" {
			t.Fatalf("unexplained omission %s", row.Path)
		}
		if _, err = os.Stat(filepath.Join(root, row.Path)); err != nil {
			t.Fatal(err)
		}
		for _, id := range row.Sources {
			if _, ok := Lookup(id); !ok {
				t.Fatalf("%s: unknown source %s", row.Path, id)
			}
		}
	}
	for _, dir := range []string{"pages", "components"} {
		err = filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".vue") || strings.Contains(path, "/components/ui/") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(data)
			repeated := false
			for _, needle := range []string{"v-for", "ItemViewSelectable", "<ItemTable", "<ListView", "<LocationTree", "<TagTree"} {
				if strings.Contains(text, needle) {
					repeated = true
				}
			}
			rel, _ := filepath.Rel(root, path)
			if repeated && !seen[rel] {
				t.Errorf("unaccounted record/control surface %s: update coverage.md and coverage-surfaces.json", rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
