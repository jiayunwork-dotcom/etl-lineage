package store

import (
	"os"
	"path/filepath"
	"testing"

	"etl-lineage/internal/graph"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func TestStoreOpenEmpty(t *testing.T) {
	s, err := Open(tmpDir(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	g := s.Graph()
	if g.NodeCount() != 0 {
		t.Fatalf("want 0 nodes, got %d", g.NodeCount())
	}
}

func TestStoreAddAndReopen(t *testing.T) {
	dir := tmpDir(t)

	// Phase 1: write data
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := s.AddNode("stg_orders", nil); err != nil {
		t.Fatalf("add node: %v", err)
	}
	attr := &graph.NodeAttr{Layer: "staging"}
	if err := s.AddNode("stg_sales", attr); err != nil {
		t.Fatalf("add node: %v", err)
	}
	if err := s.AddEdge("stg_orders", "stg_sales", &graph.EdgeAttr{Transform: graph.TransformDirect}); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	s.Close()

	// Phase 2: reopen and verify WAL replay
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	g := s2.Graph()
	if g.NodeCount() != 2 {
		t.Fatalf("want 2 nodes, got %d", g.NodeCount())
	}
	if !g.HasEdge("stg_orders", "stg_sales") {
		t.Fatal("edge stg_orders->stg_sales missing after reopen")
	}
	na, _ := g.GetNodeAttr("stg_sales")
	if na.Layer != "staging" {
		t.Fatalf("want layer=staging, got %q", na.Layer)
	}
}

func TestStoreCompactAndReopen(t *testing.T) {
	dir := tmpDir(t)

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s.AddNode("a", nil)
	s.AddNode("b", nil)
	s.AddEdge("a", "b", nil)

	if err := s.Compact(); err != nil {
		t.Fatalf("compact: %v", err)
	}
	if s.WALSize() != 0 {
		t.Fatalf("WAL should be 0 after compact, got %d", s.WALSize())
	}
	s.Close()

	// Reopen after compact: should load from snapshot
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	g := s2.Graph()
	if !g.HasEdge("a", "b") {
		t.Fatal("edge a->b missing after compact+reopen")
	}
}

func TestStoreRemoveAndReplay(t *testing.T) {
	dir := tmpDir(t)
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s.AddNode("x", nil)
	s.AddNode("y", nil)
	s.AddEdge("x", "y", nil)
	s.RemoveEdge("x", "y")
	s.RemoveNode("y")
	s.Close()

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	g := s2.Graph()
	if g.HasNode("y") {
		t.Fatal("node y should not exist after remove+replay")
	}
	if g.NodeCount() != 1 {
		t.Fatalf("want 1 node, got %d", g.NodeCount())
	}
}

func TestStoreTruncatedWALRecovery(t *testing.T) {
	dir := tmpDir(t)

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s.AddNode("good", nil)
	s.Close()

	// Corrupt WAL: append garbage
	walPath := filepath.Join(dir, walFile)
	f, _ := os.OpenFile(walPath, os.O_WRONLY|os.O_APPEND, 0o644)
	f.WriteString(`{"op":"add_node","node_id":"partial`)
	f.Close()

	// Reopen should recover up to the last valid entry
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen with truncated WAL: %v", err)
	}
	defer s2.Close()
	g := s2.Graph()
	if !g.HasNode("good") {
		t.Fatal("valid entry should survive truncated WAL")
	}
}

func TestStoreSnapshotExportImport(t *testing.T) {
	dir1 := tmpDir(t)
	s1, _ := Open(dir1)
	s1.AddNode("alpha", &graph.NodeAttr{Layer: "dim"})
	s1.AddNode("beta", nil)
	s1.AddEdge("alpha", "beta", &graph.EdgeAttr{Transform: graph.TransformAggregate})

	snap, err := s1.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	s1.Close()

	dir2 := tmpDir(t)
	s2, _ := Open(dir2)
	if err := s2.LoadSnapshot(snap); err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	defer s2.Close()

	g := s2.Graph()
	if g.NodeCount() != 2 {
		t.Fatalf("want 2, got %d", g.NodeCount())
	}
	ea, _ := g.GetEdgeAttr("alpha", "beta")
	if ea.Transform != graph.TransformAggregate {
		t.Fatalf("want aggregate, got %v", ea.Transform)
	}
}

func TestStoreSetNodeAttr(t *testing.T) {
	dir := tmpDir(t)
	s, _ := Open(dir)
	s.AddNode("n", nil)
	if err := s.SetNodeAttr("n", graph.NodeAttr{Layer: "fact", Owner: "eng"}); err != nil {
		t.Fatalf("set attr: %v", err)
	}
	s.Close()

	s2, _ := Open(dir)
	defer s2.Close()
	g := s2.Graph()
	a, _ := g.GetNodeAttr("n")
	if a.Layer != "fact" || a.Owner != "eng" {
		t.Fatalf("attr mismatch: %+v", a)
	}
}

func TestStoreAddEdgeCycleRejected(t *testing.T) {
	dir := tmpDir(t)
	s, _ := Open(dir)
	defer s.Close()
	s.AddNode("a", nil)
	s.AddNode("b", nil)
	s.AddEdge("a", "b", nil)
	if err := s.AddEdge("b", "a", nil); err == nil {
		t.Fatal("expected cycle rejection")
	}
}

func TestStoreWALGrowsOnMutations(t *testing.T) {
	dir := tmpDir(t)
	s, _ := Open(dir)
	defer s.Close()

	before := s.WALSize()
	s.AddNode("z", nil)
	after := s.WALSize()
	if after <= before {
		t.Fatalf("WAL should grow: before=%d after=%d", before, after)
	}
}
