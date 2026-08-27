package column

import (
	"fmt"
	"sort"
	"strings"
)

type FieldRef struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

func (f FieldRef) String() string {
	return f.Table + "." + f.Column
}

func ParseFieldRef(s string) (FieldRef, error) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return FieldRef{}, fmt.Errorf("invalid field ref %q (expected table.column)", s)
	}
	return FieldRef{Table: parts[0], Column: parts[1]}, nil
}

type MappingKind string

const (
	MapDirect    MappingKind = "direct"
	MapTransform MappingKind = "transform"
	MapAggregate MappingKind = "aggregate"
	MapConstant  MappingKind = "constant"
	MapDerived   MappingKind = "derived"
)

type Mapping struct {
	Source FieldRef    `json:"source"`
	Target FieldRef    `json:"target"`
	Kind   MappingKind `json:"kind"`
	Expr   string      `json:"expr,omitempty"`
}

type LineageMap struct {
	mappings []Mapping
	byTarget map[string][]int
	bySource map[string][]int
}

func NewLineageMap() *LineageMap {
	return &LineageMap{
		byTarget: make(map[string][]int),
		bySource: make(map[string][]int),
	}
}

func (lm *LineageMap) Add(m Mapping) {
	idx := len(lm.mappings)
	lm.mappings = append(lm.mappings, m)
	lm.byTarget[m.Target.String()] = append(lm.byTarget[m.Target.String()], idx)
	lm.bySource[m.Source.String()] = append(lm.bySource[m.Source.String()], idx)
}

func (lm *LineageMap) SourcesOf(target FieldRef) []Mapping {
	indices := lm.byTarget[target.String()]
	out := make([]Mapping, len(indices))
	for i, idx := range indices {
		out[i] = lm.mappings[idx]
	}
	return out
}

func (lm *LineageMap) TargetsOf(source FieldRef) []Mapping {
	indices := lm.bySource[source.String()]
	out := make([]Mapping, len(indices))
	for i, idx := range indices {
		out[i] = lm.mappings[idx]
	}
	return out
}

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

func (lm *LineageMap) AllMappings() []Mapping {
	out := make([]Mapping, len(lm.mappings))
	copy(out, lm.mappings)
	return out
}

func (lm *LineageMap) Len() int {
	return len(lm.mappings)
}

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
