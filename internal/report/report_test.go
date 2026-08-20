package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"etl-lineage/internal/graph"
)

type failWriter struct{ fail bool }

func (f failWriter) Write(p []byte) (int, error) {
	if f.fail {
		return 0, errors.New("write fail")
	}
	return len(p), nil
}

func buildPipeline() *graph.Graph {
	g := graph.New()
	g.AddNodeWithAttr("stg_orders", graph.NodeAttr{Layer: "staging", Owner: "ingest"})
	g.AddNodeWithAttr("stg_sales", graph.NodeAttr{Layer: "staging", Owner: "ingest"})
	g.AddNodeWithAttr("dim_orders", graph.NodeAttr{Layer: "dim", Owner: "analytics"})
	g.AddNodeWithAttr("fact_sales", graph.NodeAttr{Layer: "fact", Owner: "analytics"})
	g.AddNodeWithAttr("report_kpi", graph.NodeAttr{Layer: "report", Owner: "bi"})
	g.AddEdgeWithAttr("stg_orders", "dim_orders", graph.EdgeAttr{Transform: graph.TransformDirect})
	g.AddEdgeWithAttr("dim_orders", "fact_sales", graph.EdgeAttr{Transform: graph.TransformJoin})
	g.AddEdgeWithAttr("stg_sales", "fact_sales", graph.EdgeAttr{Transform: graph.TransformAggregate, Comment: "sum by day"})
	g.AddEdgeWithAttr("fact_sales", "report_kpi", graph.EdgeAttr{Transform: graph.TransformDirect})
	return g
}

func TestWriteDOTOK(t *testing.T) {
	g := buildPipeline()
	var buf bytes.Buffer
	if err := WriteDOT(&buf, g); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "digraph lineage") {
		t.Fatalf("missing digraph header: %q", out)
	}
	if !strings.Contains(out, "rankdir=LR") {
		t.Fatal("missing rankdir")
	}
	if !strings.Contains(out, `"stg_orders"`) {
		t.Fatal("missing node stg_orders")
	}
	if !strings.Contains(out, "join") {
		t.Fatal("missing edge label 'join'")
	}
}

func TestWriteDOTFlushError(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	if err := WriteDOT(failWriter{true}, g); err == nil {
		t.Fatal("expected flush error to propagate")
	}
}

func TestWriteJSONStructure(t *testing.T) {
	g := buildPipeline()
	var buf bytes.Buffer
	if err := WriteJSON(&buf, g); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	var report JSONReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if report.NodeCount != 5 {
		t.Fatalf("want 5 nodes, got %d", report.NodeCount)
	}
	if report.EdgeCount != 4 {
		t.Fatalf("want 4 edges, got %d", report.EdgeCount)
	}
	if len(report.Roots) != 2 {
		t.Fatalf("want 2 roots, got %d: %v", len(report.Roots), report.Roots)
	}
	if len(report.Leaves) != 1 || report.Leaves[0] != "report_kpi" {
		t.Fatalf("want leaves=[report_kpi], got %v", report.Leaves)
	}
	if report.Validation == nil || !report.Validation.OK {
		t.Fatal("validation should pass for healthy graph")
	}
}

func TestWriteImpactSummary(t *testing.T) {
	g := buildPipeline()
	var buf bytes.Buffer
	if err := WriteImpactSummary(&buf, g, []string{"stg_orders"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	var summary ImpactSummary
	if err := json.Unmarshal(buf.Bytes(), &summary); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if summary.ImpactedCount != 3 {
		t.Fatalf("want 3 impacted, got %d", summary.ImpactedCount)
	}
	if summary.MaxDistance != 3 {
		t.Fatalf("want max distance 3, got %d", summary.MaxDistance)
	}
}

func TestComputeImpactSummaryByLayer(t *testing.T) {
	g := buildPipeline()
	summary, err := ComputeImpactSummary(g, []string{"stg_orders"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if summary.ImpactByLayer["dim"] != 1 {
		t.Fatalf("want 1 dim impacted, got %d", summary.ImpactByLayer["dim"])
	}
	if summary.ImpactByLayer["fact"] != 1 {
		t.Fatalf("want 1 fact impacted, got %d", summary.ImpactByLayer["fact"])
	}
	if summary.ImpactByLayer["report"] != 1 {
		t.Fatalf("want 1 report impacted, got %d", summary.ImpactByLayer["report"])
	}
}

func TestWriteTextSummary(t *testing.T) {
	g := buildPipeline()
	var buf bytes.Buffer
	if err := WriteTextSummary(&buf, g); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ETL Lineage Summary") {
		t.Fatal("missing header")
	}
	if !strings.Contains(out, "Nodes: 5") {
		t.Fatal("missing node count")
	}
	if !strings.Contains(out, "Validation: PASS") {
		t.Fatal("validation should pass")
	}
}

func TestBuildDepMatrix(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	dm := BuildDepMatrix(g)
	if len(dm.Nodes) != 3 {
		t.Fatalf("want 3 nodes, got %d", len(dm.Nodes))
	}
	// Find indices
	idx := map[string]int{}
	for i, n := range dm.Nodes {
		idx[n] = i
	}
	if !dm.Matrix[idx["a"]][idx["b"]] {
		t.Fatal("a->b should be true in matrix")
	}
	if dm.Matrix[idx["a"]][idx["c"]] {
		t.Fatal("a->c should be false (not direct)")
	}
}

func TestComputeStats(t *testing.T) {
	g := buildPipeline()
	s := ComputeStats(g)
	if s.NodeCount != 5 {
		t.Fatalf("want 5, got %d", s.NodeCount)
	}
	if s.EdgeCount != 4 {
		t.Fatalf("want 4, got %d", s.EdgeCount)
	}
	if s.MaxInDegree != 2 {
		t.Fatalf("want max in-degree 2 (fact_sales), got %d", s.MaxInDegree)
	}
	if s.MaxInNode != "fact_sales" {
		t.Fatalf("want max-in node=fact_sales, got %q", s.MaxInNode)
	}
	if s.Depth < 4 {
		t.Fatalf("want depth >= 4, got %d", s.Depth)
	}
}

func TestBuildOwnerReport(t *testing.T) {
	g := buildPipeline()
	r := BuildOwnerReport(g)
	if len(r.Owners["ingest"]) != 2 {
		t.Fatalf("want 2 ingest nodes, got %d", len(r.Owners["ingest"]))
	}
	if len(r.Owners["analytics"]) != 2 {
		t.Fatalf("want 2 analytics nodes, got %d", len(r.Owners["analytics"]))
	}
	if len(r.Owners["bi"]) != 1 {
		t.Fatalf("want 1 bi node, got %d", len(r.Owners["bi"]))
	}
}

func TestWriteStatsJSON(t *testing.T) {
	g := buildPipeline()
	var buf bytes.Buffer
	if err := WriteStats(&buf, g); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	var s Stats
	if err := json.Unmarshal(buf.Bytes(), &s); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if s.NodeCount != 5 {
		t.Fatalf("want 5, got %d", s.NodeCount)
	}
}

func TestWriteDepMatrixJSON(t *testing.T) {
	g := graph.New()
	g.AddNode("x")
	g.AddNode("y")
	g.AddEdge("x", "y")
	var buf bytes.Buffer
	if err := WriteDepMatrix(&buf, g); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	var dm DepMatrix
	if err := json.Unmarshal(buf.Bytes(), &dm); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(dm.Nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(dm.Nodes))
	}
}
