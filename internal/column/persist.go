package column

import (
	"encoding/json"
	"fmt"
	"os"
)

// serialMap is the JSON-serializable form of LineageMap.
type serialMap struct {
	Mappings []Mapping `json:"mappings"`
}

// SaveLineageMap writes the column lineage map to a JSON file.
func SaveLineageMap(path string, lm *LineageMap) error {
	sm := serialMap{Mappings: lm.AllMappings()}
	data, err := json.MarshalIndent(sm, "", "  ")
	if err != nil {
		return fmt.Errorf("column: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("column: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("column: rename: %w", err)
	}
	return nil
}

// LoadLineageMap reads a column lineage map from a JSON file.
func LoadLineageMap(path string) (*LineageMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("column: read: %w", err)
	}
	var sm serialMap
	if err := json.Unmarshal(data, &sm); err != nil {
		return nil, fmt.Errorf("column: unmarshal: %w", err)
	}
	lm := NewLineageMap()
	for _, m := range sm.Mappings {
		lm.Add(m)
	}
	return lm, nil
}

// MergeLineageMaps combines multiple lineage maps into one.
// Duplicate mappings (same source, target, kind) are deduplicated.
func MergeLineageMaps(maps ...*LineageMap) *LineageMap {
	result := NewLineageMap()
	type key struct {
		src, tgt string
		kind     MappingKind
	}
	seen := map[key]bool{}
	for _, lm := range maps {
		for _, m := range lm.AllMappings() {
			k := key{m.Source.String(), m.Target.String(), m.Kind}
			if !seen[k] {
				seen[k] = true
				result.Add(m)
			}
		}
	}
	return result
}
