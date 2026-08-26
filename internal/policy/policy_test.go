package policy

import (
	"testing"

	"etl-lineage/internal/graph"
)

func TestLayerFlowRuleValid(t *testing.T) {
	g := graph.New()
	g.AddNodeWithAttr("stg", graph.NodeAttr{Layer: "staging"})
	g.AddNodeWithAttr("dim", graph.NodeAttr{Layer: "dim"})
	g.AddNodeWithAttr("fact", graph.NodeAttr{Layer: "fact"})
	g.AddEdge("stg", "dim")
	g.AddEdge("dim", "fact")

	rule := &LayerFlowRule{}
	viols := rule.Evaluate(g)
	if len(viols) != 0 {
		t.Errorf("violations = %d, want 0", len(viols))
	}
}

func TestLayerFlowRuleViolation(t *testing.T) {
	g := graph.New()
	g.AddNodeWithAttr("fact", graph.NodeAttr{Layer: "fact"})
	g.AddNodeWithAttr("stg", graph.NodeAttr{Layer: "staging"})
	g.AddEdge("fact", "stg")

	rule := &LayerFlowRule{}
	viols := rule.Evaluate(g)
	if len(viols) != 1 {
		t.Errorf("violations = %d, want 1", len(viols))
	}
	if viols[0].Severity != SeverityError {
		t.Errorf("severity = %s, want error", viols[0].Severity)
	}
}

func TestOwnerBoundaryRule(t *testing.T) {
	g := graph.New()
	g.AddNodeWithAttr("a", graph.NodeAttr{Owner: "team-a"})
	g.AddNodeWithAttr("b", graph.NodeAttr{Owner: "team-b"})
	g.AddNodeWithAttr("c", graph.NodeAttr{Owner: "team-a"})
	g.AddNodeWithAttr("d", graph.NodeAttr{Owner: "team-c"})
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("c", "d")

	rule := &OwnerBoundaryRule{MaxCrossTeam: 2}
	viols := rule.Evaluate(g)
	if len(viols) != 1 {
		t.Errorf("violations = %d, want 1 (exceeded max)", len(viols))
	}
}

func TestTransformAllowlistRule(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddEdgeWithAttr("a", "b", graph.EdgeAttr{Transform: "unknown_type"})

	rule := &TransformAllowlistRule{Allowed: []graph.TransformType{graph.TransformDirect}}
	viols := rule.Evaluate(g)
	if len(viols) != 1 {
		t.Errorf("violations = %d, want 1", len(viols))
	}
}

func TestMaxFanInRule(t *testing.T) {
	g := graph.New()
	g.AddNode("sink")
	for i := 0; i < 5; i++ {
		name := string(rune('a' + i))
		g.AddNode(name)
		g.AddEdge(name, "sink")
	}
	rule := &MaxFanInRule{MaxFanIn: 3}
	viols := rule.Evaluate(g)
	if len(viols) != 1 {
		t.Errorf("violations = %d, want 1 (sink has fan-in 5)", len(viols))
	}
}

func TestMaxFanOutRule(t *testing.T) {
	g := graph.New()
	g.AddNode("source")
	for i := 0; i < 5; i++ {
		name := string(rune('a' + i))
		g.AddNode(name)
		g.AddEdge("source", name)
	}
	rule := &MaxFanOutRule{MaxFanOut: 3}
	viols := rule.Evaluate(g)
	if len(viols) != 1 {
		t.Errorf("violations = %d, want 1 (source has fan-out 5)", len(viols))
	}
}

func TestDefaultEngineAllPass(t *testing.T) {
	g := graph.New()
	g.AddNodeWithAttr("stg", graph.NodeAttr{Layer: "staging", Owner: "team"})
	g.AddNodeWithAttr("dim", graph.NodeAttr{Layer: "dim", Owner: "team"})
	g.AddEdgeWithAttr("stg", "dim", graph.EdgeAttr{Transform: graph.TransformDirect})

	e := DefaultEngine()
	result := e.Evaluate(g)
	if !result.OK() {
		t.Errorf("expected all pass, got %d errors", len(result.Errors()))
	}
}

func TestEvalResultSummary(t *testing.T) {
	r := &EvalResult{Violations: []Violation{
		{Severity: SeverityError},
		{Severity: SeverityWarning},
	}}
	s := r.Summary()
	if s == "all policies passed" {
		t.Error("expected non-passing summary")
	}
}

func TestEngineRuleNames(t *testing.T) {
	e := DefaultEngine()
	names := e.RuleNames()
	if len(names) < 4 {
		t.Errorf("rule names = %v, want >=4", names)
	}
}
