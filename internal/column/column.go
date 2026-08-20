// Package column extends lineage tracking from table-level to column-level.
// It models how individual fields flow through ETL transformations, enabling
// fine-grained impact analysis (e.g., "which reports use customer.email?").
package column

import (
	"fmt"
	"sort"
	"strings"
)

// FieldRef identifies a column within a table.
type FieldRef struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

// String returns "table.column" representation.
func (f FieldRef) String() string {
	return f.Table + "." + f.Column
}

// ParseFieldRef parses "table.column" into a FieldRef.
func ParseFieldRef(s string) (FieldRef, error) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return FieldRef{}, fmt.Errorf("invalid field ref %q (expected table.column)", s)
	}
	return FieldRef{Table: parts[0], Column: parts[1]}, nil
}

// MappingKind describes how a source column contributes to a target column.
type MappingKind string

const (
	MapDirect    MappingKind = "direct"    // 1:1 copy
	MapTransform MappingKind = "transform" // function applied
	MapAggregate MappingKind = "aggregate" // group-by aggregation
	MapConstant  MappingKind = "constant"  // no source dependency
	MapDerived   MappingKind = "derived"   // computed from multiple sources
)

// Mapping describes one column-level dependency edge.
type Mapping struct {
	Source FieldRef    `json:"source"`
	Target FieldRef    `json:"target"`
	Kind   MappingKind `json:"kind"`
	Expr   string      `json:"expr,omitempty"` // transformation expression
}

// LineageMap holds all column-level mappings in a pipeline.
type LineageMap struct {
	mappings []Mapping
	byTarget map[string][]int // target field string -> indices into mappings
	bySource map[string][]int // source field string -> indices
}

// NewLineageMap creates an empty column lineage map.
func NewLineageMap() *LineageMap {
	return &LineageMap{
		byTarget: make(map[string][]int),
		bySource: make(map[string][]int),
	}
}

// Add registers a column mapping.
func (lm *LineageMap) Add(m Mapping) {
	idx := len(lm.mappings)
	lm.mappings = append(lm.mappings, m)
	lm.byTarget[m.Target.String()] = append(lm.byTarget[m.Target.String()], idx)
	lm.bySource[m.Source.String()] = append(lm.bySource[m.Source.String()], idx)
}

// SourcesOf returns all source columns that feed into the given target column.
func (lm *LineageMap) SourcesOf(target FieldRef) []Mapping {
	indices := lm.byTarget[target.String()]
	out := make([]Mapping, len(indices))
	for i, idx := range indices {
		out[i] = lm.mappings[idx]
	}
	return out
}

// TargetsOf returns all target columns that consume the given source column.
func (lm *LineageMap) TargetsOf(source FieldRef) []Mapping {
	indices := lm.bySource[source.String()]
	out := make([]Mapping, len(indices))
	for i, idx := range indices {
		out[i] = lm.mappings[idx]
	}
	return out
}

// TransitiveSourcesOf traces all upstream columns (recursively) for a target.
func (lm *LineageMap) TransitiveSourcesOf(target FieldRef) []FieldRef {
	visited := map[string]bool{}
	var result []FieldRef
	var walk func(FieldRef)
	walk = func(t FieldRef) {
		for _, m := range lm.SourcesOf(t) {
			key := m.Source.String()
			if !visited[key] {
				visited[key] = true
				result = append(result, m.Source)
				walk(m.Source)
			}
		}
	}
	walk(target)
	return result
}

// TransitiveTargetsOf traces all downstream columns (recursively) for a source.
func (lm *LineageMap) TransitiveTargetsOf(source FieldRef) []FieldRef {
	visited := map[string]bool{}
	var result []FieldRef
	var walk func(FieldRef)
	walk = func(s FieldRef) {
		for _, m := range lm.TargetsOf(s) {
			key := m.Target.String()
			if !visited[key] {
				visited[key] = true
				result = append(result, m.Target)
				walk(m.Target)
			}
		}
	}
	walk(source)
	return result
}

// AllMappings returns all registered mappings.
func (lm *LineageMap) AllMappings() []Mapping {
	out := make([]Mapping, len(lm.mappings))
	copy(out, lm.mappings)
	return out
}

// Len returns the number of mappings.
func (lm *LineageMap) Len() int {
	return len(lm.mappings)
}

// Tables returns all unique table names referenced in mappings.
func (lm *LineageMap) Tables() []string {
	seen := map[string]bool{}
	for _, m := range lm.mappings {
		seen[m.Source.Table] = true
		seen[m.Target.Table] = true
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// ColumnsOfTable returns all columns (source or target) for a given table.
func (lm *LineageMap) ColumnsOfTable(table string) []string {
	seen := map[string]bool{}
	for _, m := range lm.mappings {
		if m.Source.Table == table {
			seen[m.Source.Column] = true
		}
		if m.Target.Table == table {
			seen[m.Target.Column] = true
		}
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// Validate checks that no mapping references empty table/column names.
func (lm *LineageMap) Validate() []string {
	var issues []string
	for i, m := range lm.mappings {
		if m.Source.Table == "" || m.Source.Column == "" {
			issues = append(issues, fmt.Sprintf("mapping[%d]: empty source field", i))
		}
		if m.Target.Table == "" || m.Target.Column == "" {
			issues = append(issues, fmt.Sprintf("mapping[%d]: empty target field", i))
		}
	}
	return issues
}
