package lineage

import (
	"testing"

	"etl-lineage/internal/graph"
)

func buildPipeline() *graph.Graph {
	g := graph.New()
	for _, n := range []string{"stg_orders", "stg_customers", "stg_sales",
		"dim_orders", "dim_customers", "fact_sales", "report_kpi"} {
		g.AddNode(n)
	}
	g.AddEdge("stg_orders", "dim_orders")
	g.AddEdge("stg_customers", "dim_customers")
	g.AddEdge("dim_orders", "fact_sales")
	g.AddEdge("dim_customers", "fact_sales")
	g.AddEdge("stg_sales", "fact_sales")
	g.AddEdge("fact_sales", "report_kpi")
	return g
}

func buildSimple() *graph.Graph {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	return g
}

func TestUpstreamMissingNode(t *testing.T) {
	g := buildSimple()
	if _, err := Upstream(g, "ghost"); err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestUpstreamEmpty(t *testing.T) {
	g := buildSimple()
	up, err := Upstream(g, "a")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(up) != 0 {
		t.Fatalf("want 0 upstream, got %d", len(up))
	}
}

func TestUpstreamTransitive(t *testing.T) {
	g := buildSimple()
	up, err := Upstream(g, "c")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(up) != 2 || !up["a"] || !up["b"] {
		t.Fatalf("upstream of c should be {a,b}, got %v", up)
	}
}

func TestDownstreamPipeline(t *testing.T) {
	g := buildPipeline()
	down, err := Downstream(g, "stg_orders")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	expected := map[string]bool{"dim_orders": true, "fact_sales": true, "report_kpi": true}
	if len(down) != len(expected) {
		t.Fatalf("want %d downstream, got %d: %v", len(expected), len(down), down)
	}
	for k := range expected {
		if !down[k] {
			t.Fatalf("missing %q in downstream", k)
		}
	}
}

func TestBatchImpactMultipleChanges(t *testing.T) {
	g := buildPipeline()
	impact, err := BatchImpact(g, []string{"stg_orders", "stg_customers"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	expected := map[string]bool{
		"dim_orders": true, "dim_customers": true,
		"fact_sales": true, "report_kpi": true,
	}
	if len(impact) != len(expected) {
		t.Fatalf("want %d impacted, got %d: %v", len(expected), len(impact), impact)
	}
	for _, n := range impact {
		if !expected[n] {
			t.Fatalf("unexpected node %q in impact", n)
		}
	}
}

func TestBatchImpactEmpty(t *testing.T) {
	g := buildSimple()
	impact, err := BatchImpact(g, nil)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(impact) != 0 {
		t.Fatalf("want empty, got %v", impact)
	}
}

func TestBatchImpactMissing(t *testing.T) {
	g := buildSimple()
	_, err := BatchImpact(g, []string{"ghost"})
	if err == nil {
		t.Fatal("expected error for missing node")
	}
}

func TestDistanceDirect(t *testing.T) {
	g := buildSimple()
	d, err := Distance(g, "a", "c")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if d != 2 {
		t.Fatalf("want distance 2, got %d", d)
	}
}

func TestDistanceSameNode(t *testing.T) {
	g := buildSimple()
	d, err := Distance(g, "a", "a")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if d != 0 {
		t.Fatalf("want distance 0, got %d", d)
	}
}

func TestDistanceUnreachable(t *testing.T) {
	g := buildSimple()
	d, err := Distance(g, "c", "a")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if d != -1 {
		t.Fatalf("want -1 (unreachable), got %d", d)
	}
}

func TestDistanceMissingNode(t *testing.T) {
	g := buildSimple()
	_, err := Distance(g, "ghost", "a")
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestAllPathsSinglePath(t *testing.T) {
	g := buildSimple()
	paths, err := AllPaths(g, "a", "c", 0)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("want 1 path, got %d", len(paths))
	}
	if len(paths[0]) != 3 || paths[0][0] != "a" || paths[0][2] != "c" {
		t.Fatalf("unexpected path: %v", paths[0])
	}
}

func TestAllPathsMultiplePaths(t *testing.T) {
	g := buildPipeline()
	paths, err := AllPaths(g, "stg_orders", "report_kpi", 0)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("want 1 path, got %d", len(paths))
	}
	if len(paths[0]) != 4 {
		t.Fatalf("want path length 4, got %d: %v", len(paths[0]), paths[0])
	}
}

func TestAllPathsDiamond(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddNode("d")
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")
	g.AddEdge("b", "d")
	g.AddEdge("c", "d")

	paths, err := AllPaths(g, "a", "d", 0)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("want 2 paths in diamond, got %d", len(paths))
	}
}

func TestAllPathsMaxLimit(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddNode("d")
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")
	g.AddEdge("b", "d")
	g.AddEdge("c", "d")

	paths, err := AllPaths(g, "a", "d", 1)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("want at most 1 path, got %d", len(paths))
	}
}

func TestAllPathsSameNode(t *testing.T) {
	g := buildSimple()
	paths, err := AllPaths(g, "a", "a", 0)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(paths) != 1 || len(paths[0]) != 1 {
		t.Fatalf("same node path should be [[a]], got %v", paths)
	}
}

func TestLongestPathDiamond(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddNode("d")
	g.AddNode("e")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("c", "d")
	g.AddEdge("a", "d")
	g.AddEdge("d", "e")

	longest, err := LongestPath(g, "a", "e")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if longest != 4 {
		t.Fatalf("want longest path 4, got %d", longest)
	}
}

func TestLongestPathUnreachable(t *testing.T) {
	g := buildSimple()
	l, err := LongestPath(g, "c", "a")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if l != -1 {
		t.Fatalf("want -1, got %d", l)
	}
}

func TestCriticalPath(t *testing.T) {
	g := buildPipeline()
	cp, err := CriticalPath(g)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(cp) < 4 {
		t.Fatalf("critical path should be at least 4 nodes, got %d: %v", len(cp), cp)
	}
	if cp[len(cp)-1] != "report_kpi" {
		t.Fatalf("critical path should end at report_kpi, got %v", cp)
	}
}

func TestLayerOrder(t *testing.T) {
	g := buildPipeline()
	layers, err := LayerOrder(g)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(layers) < 3 {
		t.Fatalf("want at least 3 layers, got %d", len(layers))
	}
	for _, n := range layers[0] {
		preds := g.Predecessors(n)
		if len(preds) != 0 {
			t.Fatalf("layer 0 node %q has predecessors: %v", n, preds)
		}
	}
}

func TestCommonAncestors(t *testing.T) {
	g := buildPipeline()
	ca, err := CommonAncestors(g, []string{"dim_orders", "dim_customers"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(ca) != 0 {
		t.Fatalf("want 0 common ancestors, got %v", ca)
	}
}

func TestCommonAncestorsConverging(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddNode("d")
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")
	g.AddEdge("b", "d")
	g.AddEdge("c", "d")

	ca, err := CommonAncestors(g, []string{"b", "c"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(ca) != 1 || ca[0] != "a" {
		t.Fatalf("want [a], got %v", ca)
	}
}

func TestCommonAncestorsEmpty(t *testing.T) {
	g := buildSimple()
	ca, err := CommonAncestors(g, nil)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(ca) != 0 {
		t.Fatalf("want empty, got %v", ca)
	}
}
