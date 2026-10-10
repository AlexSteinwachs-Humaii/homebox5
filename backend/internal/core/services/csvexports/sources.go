package csvexports

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// Row contains only displayed data, never storage keys or credential URLs.
// Adapters supply typed numbers; text that looks numeric remains untrusted text.
type Row map[string]any

// Resolve applies captured ordering and boundaries to a complete, authorized
// read snapshot. Database adapters must apply source filters before calling it.
// An explicit boundary is the complete client-search result, not a loaded page.
func Resolve(req Request, rows []Row) ([]Row, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	source, _ := Lookup(req.Source)
	allowed := map[string]bool{}
	for _, c := range source.Columns {
		allowed[c] = true
	}
	out := make([]Row, 0, len(rows))
	excluded := map[string]bool{}
	for _, id := range req.Boundary.ExcludeIDs {
		excluded[id.String()] = true
	}
	ids := map[string]int{}
	for i, id := range req.Boundary.RecordIDs {
		ids[id.String()] = i
	}
	for _, row := range rows {
		id := fmt.Sprint(row["id"])
		if excluded[id] {
			continue
		}
		if req.Boundary.Kind == "explicit" {
			if _, ok := ids[id]; !ok {
				continue
			}
		}
		if req.Boundary.Kind == "roots" && row["parentId"] != nil && fmt.Sprint(row["parentId"]) != "" && fmt.Sprint(row["parentId"]) != uuid.Nil.String() {
			continue
		}
		projected := Row{}
		for k, v := range row {
			if allowed[k] {
				projected[k] = v
			}
		}
		out = append(out, projected)
	}
	// Preserve explicit fuzzy-search/scanner/label order when no table sort was captured.
	if req.Boundary.Kind == "explicit" && len(req.Sort) == 0 {
		sort.SliceStable(out, func(i, j int) bool { return ids[fmt.Sprint(out[i]["id"])] < ids[fmt.Sprint(out[j]["id"])] })
	} else if len(req.Sort) > 0 {
		sort.SliceStable(out, func(i, j int) bool {
			for _, s := range req.Sort {
				c := compare(out[i][s.Column], out[j][s.Column])
				if c != 0 {
					if s.Direction == "desc" {
						return c > 0
					}
					return c < 0
				}
			}
			return fmt.Sprint(out[i]["id"]) < fmt.Sprint(out[j]["id"])
		})
	}
	if req.Boundary.Kind == "recent" && len(out) > req.Boundary.Limit {
		out = out[:req.Boundary.Limit]
	}
	return out, nil
}
func compare(a, b any) int {
	if x, ok := number(a); ok {
		if y, ok := number(b); ok {
			return x.Cmp(y)
		}
	}
	return strings.Compare(fmt.Sprint(a), fmt.Sprint(b))
}
