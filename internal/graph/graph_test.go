package graph

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAddEdgeUnknownNode(t *testing.T) {
	g := New()
	if err := g.AddEdge("x", "y"); err == nil {
		t.Fatal("expected error for edge between unknown nodes")
	}
}

func TestAddEdgeCycle(t *testing.T) {
	g := New()
	g.AddNode("a")
	g.AddNode("b")
	if err := g.AddEdge("a", "b"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := g.AddEdge("b", "a"); err == nil {
		t.Fatal("expected error creating cycle")
	}
}

func TestAddEdgeSelfLoop(t *testing.T) {
	g := New()
	g.AddNode("a")
	if err := g.AddEdge("a", "a"); err == nil {
		t.Fatal("expected error for self-loop")
	}
}

func TestTopoSortOrder(t *testing.T) {
	g := New()
	g.AddNode("target")
	g.AddNode("dep1")
	g.AddNode("dep2")
	g.AddEdge("dep1", "target")
	g.AddEdge("dep2", "target")
	order, err := g.TopoSort()
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("order len=%d want 3", len(order))
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	if pos["dep1"] > pos["target"] || pos["dep2"] > pos["target"] {
		t.Fatalf("dependencies must precede target: %v", order)
	}
}

func TestTopoSortCycle(t *testing.T) {
	g := New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	if err := g.AddEdge("c", "a"); err == nil {
		t.Fatal("expected cycle detection")
	}
}

func TestEdgeAttrTransformType(t *testing.T) {
	g := New()
	g.AddNode("src")
	g.AddNode("dst")
	attr := EdgeAttr{Transform: TransformAggregate, Comment: "sum sales"}
	if err := g.AddEdgeWithAttr("src", "dst", attr); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	got, err := g.GetEdgeAttr("src", "dst")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.Transform != TransformAggregate {
		t.Fatalf("want aggregate, got %v", got.Transform)
	}
	if got.Comment != "sum sales" {
		t.Fatalf("want 'sum sales', got %q", got.Comment)
	}
}

func TestSubGraphPreservesEdges(t *testing.T) {
	g := New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("a", "c")

	sub := g.SubGraph([]string{"a", "c"})
	if sub.NodeCount() != 2 {
		t.Fatalf("want 2 nodes, got %d", sub.NodeCount())
	}
	if !sub.HasEdge("a", "c") {
		t.Fatal("subgraph should preserve edge a->c")
	}
	if sub.HasEdge("a", "b") {
		t.Fatal("subgraph should not have edge a->b (b excluded)")
	}
}

func TestRemoveNodeCleansEdges(t *testing.T) {
	g := New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	if err := g.RemoveNode("b"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if g.HasNode("b") {
		t.Fatal("node b should be removed")
	}
	if g.HasEdge("a", "b") {
		t.Fatal("edge a->b should be removed")
	}
	if g.EdgeCount() != 0 {
		t.Fatalf("want 0 edges, got %d", g.EdgeCount())
	}
}

func TestRemoveEdge(t *testing.T) {
	g := New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddEdge("a", "b")
	if err := g.RemoveEdge("a", "b"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if g.HasEdge("a", "b") {
		t.Fatal("edge should be removed")
	}
	if err := g.RemoveEdge("a", "b"); err == nil {
		t.Fatal("expected error removing non-existent edge")
	}
}

func TestRootsAndLeaves(t *testing.T) {
	g := New()
	g.AddNode("root1")
	g.AddNode("root2")
	g.AddNode("mid")
	g.AddNode("leaf")
	g.AddEdge("root1", "mid")
	g.AddEdge("root2", "mid")
	g.AddEdge("mid", "leaf")

	roots := g.Roots()
	if len(roots) != 2 || roots[0] != "root1" || roots[1] != "root2" {
		t.Fatalf("roots = %v, want [root1, root2]", roots)
	}
	leaves := g.Leaves()
	if len(leaves) != 1 || leaves[0] != "leaf" {
		t.Fatalf("leaves = %v, want [leaf]", leaves)
	}
}

func TestMarshalUnmarshalJSON(t *testing.T) {
	g := New()
	g.AddNodeWithAttr("stg", NodeAttr{Layer: "staging"})
	g.AddNodeWithAttr("dim", NodeAttr{Layer: "dim", Owner: "analytics"})
	g.AddEdgeWithAttr("stg", "dim", EdgeAttr{Transform: TransformFilter})

	data, err := g.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	g2 := New()
	if err := g2.UnmarshalJSON(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if g2.NodeCount() != 2 {
		t.Fatalf("want 2 nodes, got %d", g2.NodeCount())
	}
	attr, _ := g2.GetNodeAttr("dim")
	if attr.Owner != "analytics" {
		t.Fatalf("want owner=analytics, got %q", attr.Owner)
	}
	ea, _ := g2.GetEdgeAttr("stg", "dim")
	if ea.Transform != TransformFilter {
		t.Fatalf("want filter, got %v", ea.Transform)
	}
}

func TestWriteToReadFrom(t *testing.T) {
	g := New()
	g.AddNode("x")
	g.AddNode("y")
	g.AddEdge("x", "y")

	var buf bytes.Buffer
	if _, err := g.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}

	g2 := New()
	if _, err := g2.ReadFrom(&buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !g2.HasEdge("x", "y") {
		t.Fatal("edge x->y missing after ReadFrom")
	}
}

func TestClone(t *testing.T) {
	g := New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddEdge("a", "b")

	c := g.Clone()
	c.RemoveNode("b")
	if !g.HasNode("b") {
		t.Fatal("clone modification affected original")
	}
}

func TestNodeAttrSetAndGet(t *testing.T) {
	g := New()
	g.AddNode("n")
	if err := g.SetNodeAttr("n", NodeAttr{Layer: "fact", Owner: "team-a"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	attr, err := g.GetNodeAttr("n")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if attr.Layer != "fact" || attr.Owner != "team-a" {
		t.Fatalf("got %+v", attr)
	}
	if err := g.SetNodeAttr("ghost", NodeAttr{}); err == nil {
		t.Fatal("expected error for unknown node")
	}
}

func TestIsValidTransform(t *testing.T) {
	if !IsValidTransform(TransformJoin) {
		t.Fatal("join should be valid")
	}
	if IsValidTransform("magic") {
		t.Fatal("magic should not be valid")
	}
}

func TestUnmarshalBadEdgeRef(t *testing.T) {
	raw := `{"nodes":[{"id":"a","attr":{}}],"edges":[{"from":"a","to":"z","attr":{"transform":"direct"}}]}`
	g := New()
	if err := json.Unmarshal([]byte(raw), g); err == nil {
		t.Fatal("expected error for edge referencing unknown node")
	}
}
