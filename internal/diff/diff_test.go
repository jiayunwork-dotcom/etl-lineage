package diff

import (
	"testing"

	"etl-lineage/internal/graph"
)

func buildGraph(nodes []string, edges [][2]string) *graph.Graph {
	g := graph.New()
	for _, n := range nodes {
		g.AddNode(n)
	}
	for _, e := range edges {
		g.AddEdge(e[0], e[1])
	}
	return g
}

func TestCompareIdentical(t *testing.T) {
	g := buildGraph([]string{"a", "b"}, [][2]string{{"a", "b"}})
	d := Compare(g, g)
	if !d.IsEmpty() {
		t.Errorf("identical graphs should have no changes, got %d", len(d.Changes))
	}
}

func TestCompareAddedNode(t *testing.T) {
	old := buildGraph([]string{"a"}, nil)
	new := buildGraph([]string{"a", "b"}, nil)
	d := Compare(old, new)
	if d.Added != 1 {
		t.Errorf("Added = %d, want 1", d.Added)
	}
	adds := d.FilterByKind(ChangeAddNode)
	if len(adds) != 1 || adds[0].NodeID != "b" {
		t.Errorf("unexpected add: %+v", adds)
	}
}

func TestCompareRemovedNode(t *testing.T) {
	old := buildGraph([]string{"a", "b"}, nil)
	new := buildGraph([]string{"a"}, nil)
	d := Compare(old, new)
	if d.Removed != 1 {
		t.Errorf("Removed = %d, want 1", d.Removed)
	}
}

func TestCompareModifiedNodeAttr(t *testing.T) {
	old := graph.New()
	old.AddNodeWithAttr("x", graph.NodeAttr{Layer: "staging"})
	new := graph.New()
	new.AddNodeWithAttr("x", graph.NodeAttr{Layer: "fact"})

	d := Compare(old, new)
	if d.Modified != 1 {
		t.Errorf("Modified = %d, want 1", d.Modified)
	}
}

func TestCompareAddedEdge(t *testing.T) {
	old := buildGraph([]string{"a", "b"}, nil)
	new := buildGraph([]string{"a", "b"}, [][2]string{{"a", "b"}})
	d := Compare(old, new)
	adds := d.FilterByKind(ChangeAddEdge)
	if len(adds) != 1 {
		t.Errorf("added edges = %d, want 1", len(adds))
	}
}

func TestCompareRemovedEdge(t *testing.T) {
	old := buildGraph([]string{"a", "b"}, [][2]string{{"a", "b"}})
	new := buildGraph([]string{"a", "b"}, nil)
	d := Compare(old, new)
	removes := d.FilterByKind(ChangeRemoveEdge)
	if len(removes) != 1 {
		t.Errorf("removed edges = %d, want 1", len(removes))
	}
}

func TestAffectedNodes(t *testing.T) {
	old := buildGraph([]string{"a", "b"}, nil)
	new := buildGraph([]string{"a", "b", "c"}, [][2]string{{"a", "c"}})
	d := Compare(old, new)
	affected := d.AffectedNodes()
	if len(affected) < 2 {
		t.Errorf("affected = %v, want >= 2", affected)
	}
}

func TestMergeNoConflict(t *testing.T) {
	ours := buildGraph([]string{"a", "b"}, [][2]string{{"a", "b"}})
	theirs := buildGraph([]string{"a", "c"}, [][2]string{{"a", "c"}})
	mr := Merge(ours, theirs)
	if mr.HasConflicts() {
		t.Errorf("unexpected conflicts: %+v", mr.Conflicts)
	}
	if !mr.Graph.HasNode("b") || !mr.Graph.HasNode("c") {
		t.Error("merged graph missing nodes")
	}
}

func TestMergeAttrConflict(t *testing.T) {
	ours := graph.New()
	ours.AddNodeWithAttr("x", graph.NodeAttr{Layer: "staging"})
	theirs := graph.New()
	theirs.AddNodeWithAttr("x", graph.NodeAttr{Layer: "fact"})

	mr := Merge(ours, theirs)
	if !mr.HasConflicts() {
		t.Error("expected attribute conflict")
	}
	attr, _ := mr.Graph.GetNodeAttr("x")
	if attr.Layer != "fact" {
		t.Errorf("attr.Layer = %q, want fact", attr.Layer)
	}
}

func TestThreeWayMergeOneSideNoChange(t *testing.T) {
	base := buildGraph([]string{"a", "b"}, [][2]string{{"a", "b"}})
	ours := buildGraph([]string{"a", "b"}, [][2]string{{"a", "b"}})
	theirs := buildGraph([]string{"a", "b", "c"}, [][2]string{{"a", "b"}, {"b", "c"}})

	mr := ThreeWayMerge(base, ours, theirs)
	if mr.HasConflicts() {
		t.Errorf("unexpected conflicts: %+v", mr.Conflicts)
	}
	if !mr.Graph.HasNode("c") {
		t.Error("expected theirs change (node c) in result")
	}
}
