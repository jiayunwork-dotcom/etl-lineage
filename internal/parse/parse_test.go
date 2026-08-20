package parse

import (
	"bytes"
	"strings"
	"testing"

	"etl-lineage/internal/graph"
)

func TestParseSpecOK(t *testing.T) {
	in := "a <- b, c\nb <- c\n"
	g, err := ParseSpec(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !g.HasNode("a") || !g.HasNode("b") || !g.HasNode("c") {
		t.Fatalf("nodes missing: %v", g.Nodes())
	}
	if len(g.Successors("c")) != 2 {
		t.Fatalf("c should have 2 outgoing edges, got %v", g.Successors("c"))
	}
}

func TestParseSpecMissingSeparator(t *testing.T) {
	if _, err := ParseSpec(strings.NewReader("a b c\n")); err == nil {
		t.Fatal("expected error for missing '<-' separator")
	}
}

func TestParseSpecEmptyTarget(t *testing.T) {
	if _, err := ParseSpec(strings.NewReader(" <- a\n")); err == nil {
		t.Fatal("expected error for empty target")
	}
}

func TestParseSpecWithTransform(t *testing.T) {
	in := "fact_sales <-[aggregate] stg_sales\n"
	g, err := ParseSpec(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	ea, err := g.GetEdgeAttr("stg_sales", "fact_sales")
	if err != nil {
		t.Fatalf("edge not found: %v", err)
	}
	if ea.Transform != graph.TransformAggregate {
		t.Fatalf("want aggregate, got %v", ea.Transform)
	}
}

func TestParseSpecInvalidTransform(t *testing.T) {
	in := "x <-[magic] y\n"
	_, err := ParseSpec(strings.NewReader(in))
	if err == nil {
		t.Fatal("expected error for invalid transform")
	}
}

func TestParseSpecUnclosedBracket(t *testing.T) {
	in := "x <-[join y\n"
	_, err := ParseSpec(strings.NewReader(in))
	if err == nil {
		t.Fatal("expected error for unclosed bracket")
	}
}

func TestParseSpecAttrDirective(t *testing.T) {
	in := "@stg_orders layer=staging owner=ingest\nstg_orders <-\n"
	g, err := ParseSpec(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	attr, err := g.GetNodeAttr("stg_orders")
	if err != nil {
		t.Fatalf("node not found: %v", err)
	}
	if attr.Layer != "staging" {
		t.Fatalf("want layer=staging, got %q", attr.Layer)
	}
	if attr.Owner != "ingest" {
		t.Fatalf("want owner=ingest, got %q", attr.Owner)
	}
}

func TestParseSpecAttrBadKey(t *testing.T) {
	in := "@node color=red\n"
	_, err := ParseSpec(strings.NewReader(in))
	if err == nil {
		t.Fatal("expected error for unknown attribute key")
	}
}

func TestParseSpecAttrMissingValue(t *testing.T) {
	in := "@node layer\n"
	_, err := ParseSpec(strings.NewReader(in))
	if err == nil {
		t.Fatal("expected error for missing = in attribute")
	}
}

func TestParseSpecCommentsAndBlanks(t *testing.T) {
	in := "# this is a comment\n\na <- b\n\n# another\nb <-\n"
	g, err := ParseSpec(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if g.NodeCount() != 2 {
		t.Fatalf("want 2 nodes, got %d", g.NodeCount())
	}
}

func TestParseSpecCycleRejected(t *testing.T) {
	in := "a <- b\nb <- a\n"
	_, err := ParseSpec(strings.NewReader(in))
	if err == nil {
		t.Fatal("expected error for cycle")
	}
}

func TestFormatSpecRoundTrip(t *testing.T) {
	in := "@stg layer=staging\n@dim layer=dim\nstg <-\ndim <- stg\n"
	g, err := ParseSpec(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var buf bytes.Buffer
	if err := FormatSpec(&buf, g); err != nil {
		t.Fatalf("format: %v", err)
	}

	// Re-parse the formatted output
	g2, err := ParseSpec(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if g2.NodeCount() != g.NodeCount() {
		t.Fatalf("round-trip node count: %d vs %d", g.NodeCount(), g2.NodeCount())
	}
	if g2.EdgeCount() != g.EdgeCount() {
		t.Fatalf("round-trip edge count: %d vs %d", g.EdgeCount(), g2.EdgeCount())
	}
}
