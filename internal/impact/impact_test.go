package impact

import (
	"testing"

	"etl-lineage/internal/graph"
)

func build() *graph.Graph {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	return g
}

func TestImpactSorted(t *testing.T) {
	g := build()
	got, err := Impact(g, "a")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := []string{"b", "c"}
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Fatalf("impact of a = %v, want %v", got, want)
	}
}

func TestImpactMissingNode(t *testing.T) {
	g := build()
	if _, err := Impact(g, "ghost"); err == nil {
		t.Fatal("expected error for missing node")
	}
}
