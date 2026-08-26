package validate

import (
	"testing"

	"etl-lineage/internal/graph"
)

func buildHealthy() *graph.Graph {
	g := graph.New()
	g.AddNodeWithAttr("stg_orders", graph.NodeAttr{Layer: "staging"})
	g.AddNodeWithAttr("dim_orders", graph.NodeAttr{Layer: "dim"})
	g.AddNodeWithAttr("fact_sales", graph.NodeAttr{Layer: "fact"})
	g.AddEdgeWithAttr("stg_orders", "dim_orders", graph.EdgeAttr{Transform: graph.TransformDirect})
	g.AddEdgeWithAttr("dim_orders", "fact_sales", graph.EdgeAttr{Transform: graph.TransformJoin})
	return g
}

func TestValidateHealthyGraph(t *testing.T) {
	g := buildHealthy()
	r := Validate(g)
	if !r.OK() {
		t.Fatalf("healthy graph should pass, got errors: %v", r.Errors())
	}
}

func TestValidateOrphanNode(t *testing.T) {
	g := buildHealthy()
	g.AddNode("orphan")
	r := Validate(g)
	found := false
	for _, iss := range r.Warnings() {
		if iss.Code == "orphan_node" && iss.Node == "orphan" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected orphan_node warning for 'orphan'")
	}
}

func TestValidateInvalidTransform(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddEdgeWithAttr("a", "b", graph.EdgeAttr{Transform: "magic"})
	r := Validate(g)
	found := false
	for _, iss := range r.Errors() {
		if iss.Code == "invalid_transform" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected invalid_transform error")
	}
}

func TestValidateLayerViolation(t *testing.T) {
	g := graph.New()
	g.AddNodeWithAttr("fact_x", graph.NodeAttr{Layer: "fact"})
	g.AddNodeWithAttr("stg_y", graph.NodeAttr{Layer: "staging"})
	g.AddEdgeWithAttr("fact_x", "stg_y", graph.EdgeAttr{Transform: graph.TransformDirect})
	r := Validate(g)
	found := false
	for _, iss := range r.Errors() {
		if iss.Code == "layer_violation" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected layer_violation error for fact->staging edge")
	}
}

func TestValidateDisconnectedComponents(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddEdge("a", "b")
	g.AddNode("c")
	g.AddNode("d")
	g.AddEdge("c", "d")
	r := Validate(g)
	found := false
	for _, iss := range r.Warnings() {
		if iss.Code == "disconnected" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected disconnected warning for 2-component graph")
	}
}

func TestDetectOrphans(t *testing.T) {
	g := graph.New()
	g.AddNode("connected")
	g.AddNode("also_connected")
	g.AddEdge("connected", "also_connected")
	g.AddNode("lonely")

	orphans := DetectOrphans(g)
	if len(orphans) != 1 || orphans[0] != "lonely" {
		t.Fatalf("want [lonely], got %v", orphans)
	}
}

func TestDetectOrphansSingleNode(t *testing.T) {
	g := graph.New()
	g.AddNode("only")
	orphans := DetectOrphans(g)
	if len(orphans) != 0 {
		t.Fatalf("single-node graph should have no orphans, got %v", orphans)
	}
}

func TestCheckDuplicateEdgesNone(t *testing.T) {
	g := buildHealthy()
	issues := CheckDuplicateEdges(g)
	if len(issues) != 0 {
		t.Fatalf("want no duplicates, got %v", issues)
	}
}

func TestValidateNodeNamingOK(t *testing.T) {
	g := graph.New()
	g.AddNode("stg_orders")
	g.AddNode("dim-customers")
	g.AddNode("fact123")
	issues := ValidateNodeNaming(g)
	if len(issues) != 0 {
		t.Fatalf("want no naming issues, got %v", issues)
	}
}

func TestValidateNodeNamingBad(t *testing.T) {
	g := graph.New()
	g.AddNode("BadName")
	g.AddNode("has space")
	issues := ValidateNodeNaming(g)
	if len(issues) != 2 {
		t.Fatalf("want 2 naming issues, got %d: %v", len(issues), issues)
	}
}

func TestValidateCompleteAggregates(t *testing.T) {
	g := graph.New()
	g.AddNodeWithAttr("Stg_X", graph.NodeAttr{Layer: "staging"})
	g.AddNodeWithAttr("dim_y", graph.NodeAttr{Layer: "dim"})
	g.AddEdgeWithAttr("Stg_X", "dim_y", graph.EdgeAttr{Transform: graph.TransformFilter})
	r := ValidateComplete(g)
	found := false
	for _, iss := range r.Issues {
		if iss.Code == "naming_violation" && iss.Node == "Stg_X" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected naming_violation for Stg_X")
	}
}

func TestResultMethods(t *testing.T) {
	r := &Result{
		Issues: []Issue{
			{Severity: "error", Code: "a"},
			{Severity: "warning", Code: "b"},
			{Severity: "error", Code: "c"},
		},
	}
	if r.OK() {
		t.Fatal("should not be OK with errors")
	}
	if len(r.Errors()) != 2 {
		t.Fatalf("want 2 errors, got %d", len(r.Errors()))
	}
	if len(r.Warnings()) != 1 {
		t.Fatalf("want 1 warning, got %d", len(r.Warnings()))
	}
}
