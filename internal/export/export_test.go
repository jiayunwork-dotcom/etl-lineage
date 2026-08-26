package export

import (
	"bytes"
	"strings"
	"testing"

	"etl-lineage/internal/graph"
)

func sampleGraph() *graph.Graph {
	g := graph.New()
	g.AddNodeWithAttr("stg_orders", graph.NodeAttr{Layer: "staging", Owner: "data-eng"})
	g.AddNodeWithAttr("dim_customer", graph.NodeAttr{Layer: "dim", Owner: "data-eng"})
	g.AddNodeWithAttr("fact_sales", graph.NodeAttr{Layer: "fact", Owner: "analytics"})
	g.AddEdgeWithAttr("stg_orders", "dim_customer", graph.EdgeAttr{Transform: graph.TransformDirect})
	g.AddEdgeWithAttr("stg_orders", "fact_sales", graph.EdgeAttr{Transform: graph.TransformAggregate, Comment: "sum by day"})
	g.AddEdgeWithAttr("dim_customer", "fact_sales", graph.EdgeAttr{Transform: graph.TransformJoin})
	return g
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, sampleGraph()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "from,to,transform,comment") {
		t.Error("missing CSV header")
	}
	if !strings.Contains(out, "stg_orders") {
		t.Error("missing edge data")
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Errorf("lines = %d, want 4", len(lines))
	}
}

func TestWriteYAML(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteYAML(&buf, sampleGraph()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "nodes:") {
		t.Error("missing nodes section")
	}
	if !strings.Contains(out, "edges:") {
		t.Error("missing edges section")
	}
	if !strings.Contains(out, "layer: staging") {
		t.Error("missing layer attr")
	}
}

func TestWriteSQLDDL(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSQLDDL(&buf, sampleGraph()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "CREATE TABLE") {
		t.Error("missing CREATE TABLE")
	}
	if !strings.Contains(out, "INSERT INTO lineage_nodes") {
		t.Error("missing INSERT nodes")
	}
	if !strings.Contains(out, "INSERT INTO lineage_edges") {
		t.Error("missing INSERT edges")
	}
}

func TestWriteNodeList(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteNodeList(&buf, sampleGraph()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("lines = %d, want 3", len(lines))
	}
}

func TestWriteGraphML(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteGraphML(&buf, sampleGraph()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<graphml") {
		t.Error("missing graphml root element")
	}
	if !strings.Contains(out, "stg_orders") {
		t.Error("missing node data")
	}
}

func TestWriteGEXF(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteGEXF(&buf, sampleGraph()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<gexf") {
		t.Error("missing gexf root")
	}
	if !strings.Contains(out, "<node") {
		t.Error("missing node elements")
	}
	if !strings.Contains(out, "<edge") {
		t.Error("missing edge elements")
	}
}

func TestEscapeSQLStr(t *testing.T) {
	if s := escapeSQLStr("it's a test"); s != "it''s a test" {
		t.Errorf("escaped = %q", s)
	}
}
