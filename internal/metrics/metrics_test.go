package metrics

import (
	"testing"

	"etl-lineage/internal/graph"
)

func pipelineGraph() *graph.Graph {
	g := graph.New()
	g.AddNodeWithAttr("src1", graph.NodeAttr{Layer: "staging", Owner: "team-a"})
	g.AddNodeWithAttr("src2", graph.NodeAttr{Layer: "staging", Owner: "team-a"})
	g.AddNodeWithAttr("dim", graph.NodeAttr{Layer: "dim", Owner: "team-b"})
	g.AddNodeWithAttr("fact", graph.NodeAttr{Layer: "fact", Owner: "team-b"})
	g.AddNodeWithAttr("report", graph.NodeAttr{Layer: "report", Owner: "team-c"})
	g.AddEdge("src1", "dim")
	g.AddEdge("src2", "dim")
	g.AddEdge("dim", "fact")
	g.AddEdge("fact", "report")
	return g
}

func TestComputeBasic(t *testing.T) {
	m := Compute(pipelineGraph())
	if m.NodeCount != 5 {
		t.Errorf("NodeCount = %d, want 5", m.NodeCount)
	}
	if m.EdgeCount != 4 {
		t.Errorf("EdgeCount = %d, want 4", m.EdgeCount)
	}
	if m.RootCount != 2 {
		t.Errorf("RootCount = %d, want 2", m.RootCount)
	}
	if m.LeafCount != 1 {
		t.Errorf("LeafCount = %d, want 1", m.LeafCount)
	}
}

func TestComputeDepth(t *testing.T) {
	m := Compute(pipelineGraph())
	if m.Depth != 3 {
		t.Errorf("Depth = %d, want 3", m.Depth)
	}
}

func TestComputeMaxFanIn(t *testing.T) {
	m := Compute(pipelineGraph())
	if m.MaxFanIn != 2 {
		t.Errorf("MaxFanIn = %d, want 2", m.MaxFanIn)
	}
	if m.MaxFanInNode != "dim" {
		t.Errorf("MaxFanInNode = %s, want dim", m.MaxFanInNode)
	}
}

func TestComputeDensity(t *testing.T) {
	m := Compute(pipelineGraph())
	if m.Density < 0.19 || m.Density > 0.21 {
		t.Errorf("Density = %f, want ~0.2", m.Density)
	}
}

func TestComputeComponents(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddEdge("a", "b")
	m := Compute(g)
	if m.Components != 2 {
		t.Errorf("Components = %d, want 2", m.Components)
	}
}

func TestComputeCoupling(t *testing.T) {
	m := Compute(pipelineGraph())
	if m.CouplingScore < 0.7 || m.CouplingScore > 0.8 {
		t.Errorf("CouplingScore = %f, want ~0.75", m.CouplingScore)
	}
}

func TestComplexityScore(t *testing.T) {
	m := Compute(pipelineGraph())
	score := m.ComplexityScore()
	if score < 0 || score > 100 {
		t.Errorf("ComplexityScore = %f, out of range", score)
	}
}

func TestHotspots(t *testing.T) {
	g := pipelineGraph()
	spots := Hotspots(g, 2)
	if len(spots) != 2 {
		t.Fatalf("hotspots = %v", spots)
	}
	if spots[0] != "dim" {
		t.Errorf("top hotspot = %s, want dim", spots[0])
	}
}

func TestCheckHealthGood(t *testing.T) {
	g := graph.New()
	g.AddNodeWithAttr("stg", graph.NodeAttr{Layer: "staging", Owner: "team"})
	g.AddNodeWithAttr("dim", graph.NodeAttr{Layer: "dim", Owner: "team"})
	g.AddNodeWithAttr("fact", graph.NodeAttr{Layer: "fact", Owner: "team"})
	g.AddEdge("stg", "dim")
	g.AddEdge("dim", "fact")

	cfg := &HealthConfig{MaxDepth: 10, MaxFanIn: 10, MaxFanOut: 10, MaxDensity: 0.5, MaxCoupling: 1.0, MaxComponents: 5}
	report := CheckHealth(g, cfg)
	if report.Overall != HealthGood {
		t.Errorf("overall = %s, want GOOD; checks: %+v", report.Overall, report.Checks)
	}
}

func TestCheckHealthCritical(t *testing.T) {
	g := graph.New()
	prev := "n0"
	g.AddNode(prev)
	for i := 1; i <= 15; i++ {
		name := "n" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		g.AddNode(name)
		g.AddEdge(prev, name)
		prev = name
	}
	cfg := &HealthConfig{MaxDepth: 5, MaxFanIn: 10, MaxFanOut: 10, MaxDensity: 0.5, MaxCoupling: 1.0, MaxComponents: 5}
	report := CheckHealth(g, cfg)
	if report.Overall == HealthGood {
		t.Error("expected non-GOOD for deep pipeline")
	}
}

func TestHealthReportPassingChecks(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	report := CheckHealth(g, nil)
	if report.PassingChecks() < 1 {
		t.Error("expected at least 1 passing check")
	}
}

func TestSummaryFormat(t *testing.T) {
	m := Compute(pipelineGraph())
	s := m.Summary()
	if s == "" {
		t.Error("empty summary")
	}
}
