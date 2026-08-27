package policy

import (
	"fmt"
	"sort"

	"etl-lineage/internal/graph"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type Violation struct {
	Rule     string    `json:"rule"`
	Severity Severity  `json:"severity"`
	Message  string    `json:"message"`
	Nodes    []string  `json:"nodes,omitempty"`
	Edge     [2]string `json:"edge,omitempty"`
}

type EvalResult struct {
	Violations []Violation `json:"violations"`
}

func (r *EvalResult) OK() bool {
	for _, v := range r.Violations {
		if v.Severity == SeverityError {
			return false
		}
	}
	return true
}

func (r *EvalResult) Errors() []Violation {
	var out []Violation
	for _, v := range r.Violations {
		if v.Severity == SeverityError {
			out = append(out, v)
		}
	}
	return out
}

func (r *EvalResult) Count() int {
	return len(r.Violations)
}

type Rule interface {
	Name() string
	Evaluate(g *graph.Graph) []Violation
}

type Engine struct {
	rules []Rule
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) AddRule(r Rule) {
	e.rules = append(e.rules, r)
}

func (e *Engine) Evaluate(g *graph.Graph) *EvalResult {
	result := &EvalResult{}
	for _, rule := range e.rules {
		viols := rule.Evaluate(g)
		result.Violations = append(result.Violations, viols...)
	}
	return result
}

func (e *Engine) RuleNames() []string {
	names := make([]string, len(e.rules))
	for i, r := range e.rules {
		names[i] = r.Name()
	}
	return names
}

func DefaultEngine() *Engine {
	e := NewEngine()
	e.AddRule(&LayerFlowRule{})
	e.AddRule(&OwnerBoundaryRule{MaxCrossTeam: 3})
	e.AddRule(&TransformAllowlistRule{
		Allowed: []graph.TransformType{
			graph.TransformDirect, graph.TransformFilter,
			graph.TransformAggregate, graph.TransformJoin,
			graph.TransformLookup, graph.TransformUnion,
		},
	})
	e.AddRule(&MaxFanInRule{MaxFanIn: 10})
	e.AddRule(&MaxFanOutRule{MaxFanOut: 20})
	return e
}

func (r *EvalResult) Summary() string {
	if r.OK() && len(r.Violations) == 0 {
		return "all policies passed"
	}
	errors := 0
	warnings := 0
	for _, v := range r.Violations {
		switch v.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		}
	}
	return fmt.Sprintf("%d violation(s): %d errors, %d warnings", len(r.Violations), errors, warnings)
}

func (r *EvalResult) AffectedNodes() []string {
	seen := map[string]bool{}
	for _, v := range r.Violations {
		for _, n := range v.Nodes {
			seen[n] = true
		}
		if v.Edge[0] != "" {
			seen[v.Edge[0]] = true
		}
		if v.Edge[1] != "" {
			seen[v.Edge[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
