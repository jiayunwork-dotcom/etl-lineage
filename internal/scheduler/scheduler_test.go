package scheduler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"etl-lineage/internal/graph"
)

func linearGraph() *graph.Graph {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	return g
}

func diamondGraph() *graph.Graph {
	g := graph.New()
	g.AddNode("root")
	g.AddNode("left")
	g.AddNode("right")
	g.AddNode("sink")
	g.AddEdge("root", "left")
	g.AddEdge("root", "right")
	g.AddEdge("left", "sink")
	g.AddEdge("right", "sink")
	return g
}

func TestSchedulerLinear(t *testing.T) {
	s, err := New(linearGraph())
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	results := s.Run(context.Background(), func(_ context.Context, id string) error {
		order = append(order, id)
		return nil
	})
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Errorf("order = %v, want [a b c]", order)
	}
	for _, r := range results {
		if r.Status != StatusDone {
			t.Errorf("%s status = %s, want done", r.NodeID, r.Status)
		}
	}
}

func TestSchedulerFailureSkipsDownstream(t *testing.T) {
	s, _ := New(linearGraph())
	results := s.Run(context.Background(), func(_ context.Context, id string) error {
		if id == "b" {
			return fmt.Errorf("b failed")
		}
		return nil
	})
	if results["a"].Status != StatusDone {
		t.Errorf("a = %s, want done", results["a"].Status)
	}
	if results["b"].Status != StatusFailed {
		t.Errorf("b = %s, want failed", results["b"].Status)
	}
	if results["c"].Status != StatusSkipped {
		t.Errorf("c = %s, want skipped", results["c"].Status)
	}
}

func TestSchedulerCancellation(t *testing.T) {
	g := graph.New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")

	s, _ := New(g)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	results := s.Run(ctx, func(ctx context.Context, id string) error {
		if id == "a" {
			// a takes long enough for ctx to expire
			time.Sleep(50 * time.Millisecond)
		}
		return nil
	})
	// a may complete (started before cancel), b and c should be cancelled
	if results["a"].Status != StatusDone {
		// a was already running when cancel fired
		t.Logf("a status = %s (may have completed before ctx expired)", results["a"].Status)
	}
	// at least one of b/c should be cancelled or skipped
	bStatus := results["b"].Status
	cStatus := results["c"].Status
	if bStatus != StatusCancelled && bStatus != StatusSkipped {
		t.Errorf("b = %s, want cancelled/skipped", bStatus)
	}
	if cStatus != StatusCancelled && cStatus != StatusSkipped {
		t.Errorf("c = %s, want cancelled/skipped", cStatus)
	}
}

func TestSchedulerCompletedResultsPreserved(t *testing.T) {
	s, _ := New(linearGraph())
	results := s.Run(context.Background(), func(_ context.Context, id string) error {
		if id == "b" {
			return fmt.Errorf("fail")
		}
		return nil
	})
	// a completed before b failed - its result must be preserved
	if results["a"].Status != StatusDone {
		t.Errorf("completed stage a not preserved: %s", results["a"].Status)
	}
	if results["a"].Duration <= 0 {
		t.Error("expected positive duration for completed stage")
	}
}

func TestSchedulerDiamond(t *testing.T) {
	s, _ := New(diamondGraph())
	var executed []string
	results := s.Run(context.Background(), func(_ context.Context, id string) error {
		executed = append(executed, id)
		return nil
	})
	if len(results) != 4 {
		t.Fatalf("results = %d", len(results))
	}
	// root must be first, sink must be last
	if executed[0] != "root" {
		t.Errorf("first = %s, want root", executed[0])
	}
	if executed[3] != "sink" {
		t.Errorf("last = %s, want sink", executed[3])
	}
}

func TestSummary(t *testing.T) {
	results := map[string]*TaskResult{
		"a": {Status: StatusDone},
		"b": {Status: StatusFailed},
		"c": {Status: StatusSkipped},
	}
	s := Summary(results)
	if s[StatusDone] != 1 || s[StatusFailed] != 1 || s[StatusSkipped] != 1 {
		t.Errorf("summary = %v", s)
	}
}

func TestPrioritizedOrderFanOut(t *testing.T) {
	g := graph.New()
	g.AddNode("root")
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	g.AddNode("d")
	g.AddEdge("root", "a")
	g.AddEdge("root", "b")
	g.AddEdge("a", "c")
	g.AddEdge("a", "d")

	order, err := PrioritizedOrder(g, PriorityFanOut)
	if err != nil {
		t.Fatal(err)
	}
	if order[0] != "root" {
		t.Errorf("first = %s, want root", order[0])
	}
}

func TestEstimateCriticalPathLength(t *testing.T) {
	g := linearGraph() // a->b->c, length=2
	length, err := EstimateCriticalPathLength(g)
	if err != nil {
		t.Fatal(err)
	}
	if length != 2 {
		t.Errorf("critical path = %d, want 2", length)
	}
}
