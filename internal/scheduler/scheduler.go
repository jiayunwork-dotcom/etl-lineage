// Package scheduler provides topological execution scheduling for ETL lineage
// graphs. It supports context cancellation with consistent partial results:
// completed stages are never invalidated by downstream failures or cancellation.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"etl-lineage/internal/graph"
)

// TaskStatus represents the current state of a scheduled task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusDone      TaskStatus = "done"
	StatusFailed    TaskStatus = "failed"
	StatusSkipped   TaskStatus = "skipped"
	StatusCancelled TaskStatus = "cancelled"
)

// TaskResult holds the outcome of executing a single node's task.
type TaskResult struct {
	NodeID    string        `json:"node_id"`
	Status    TaskStatus    `json:"status"`
	StartedAt time.Time    `json:"started_at"`
	EndedAt   time.Time    `json:"ended_at"`
	Duration  time.Duration `json:"duration"`
	Error     error         `json:"-"`
}

// TaskFunc is the function executed for each node. It receives the node ID
// and a context that will be cancelled if the pipeline is aborted.
type TaskFunc func(ctx context.Context, nodeID string) error

// Scheduler executes tasks in topological order respecting the DAG dependencies.
// If a task fails, all downstream tasks are skipped. If the context is cancelled,
// pending tasks are marked as cancelled. Already-completed tasks remain valid.
type Scheduler struct {
	mu      sync.Mutex
	g       *graph.Graph
	results map[string]*TaskResult
	order   []string
}

// New creates a scheduler for the given graph. Returns error if graph has a cycle.
func New(g *graph.Graph) (*Scheduler, error) {
	order, err := g.TopoSort()
	if err != nil {
		return nil, fmt.Errorf("scheduler: %w", err)
	}
	return &Scheduler{
		g:       g,
		order:   order,
		results: make(map[string]*TaskResult),
	}, nil
}

// Run executes the task function for each node in topological order.
// Dependencies are respected: a node runs only after all its predecessors
// have completed successfully. If any predecessor failed, the node is skipped.
// Cancellation via ctx stops scheduling new tasks but does not interrupt running ones.
func (s *Scheduler) Run(ctx context.Context, fn TaskFunc) map[string]*TaskResult {
	s.mu.Lock()
	s.results = make(map[string]*TaskResult, len(s.order))
	s.mu.Unlock()

	for _, nodeID := range s.order {
		select {
		case <-ctx.Done():
			s.markRemaining(nodeID, StatusCancelled)
			return s.copyResults()
		default:
		}

		// Check if any predecessor failed or was skipped
		if !s.allPredsOK(nodeID) {
			s.setResult(nodeID, &TaskResult{
				NodeID:    nodeID,
				Status:    StatusSkipped,
				StartedAt: time.Now(),
				EndedAt:   time.Now(),
			})
			continue
		}

		start := time.Now()
		s.setResult(nodeID, &TaskResult{
			NodeID:    nodeID,
			Status:    StatusRunning,
			StartedAt: start,
		})

		err := fn(ctx, nodeID)
		end := time.Now()

		if err != nil {
			s.setResult(nodeID, &TaskResult{
				NodeID:    nodeID,
				Status:    StatusFailed,
				StartedAt: start,
				EndedAt:   end,
				Duration:  end.Sub(start),
				Error:     err,
			})
		} else {
			s.setResult(nodeID, &TaskResult{
				NodeID:    nodeID,
				Status:    StatusDone,
				StartedAt: start,
				EndedAt:   end,
				Duration:  end.Sub(start),
			})
		}
	}

	return s.copyResults()
}

// Results returns a copy of current results.
func (s *Scheduler) Results() map[string]*TaskResult {
	return s.copyResults()
}

// Order returns the topological execution order.
func (s *Scheduler) Order() []string {
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

func (s *Scheduler) allPredsOK(nodeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, pred := range s.g.Predecessors(nodeID) {
		r, ok := s.results[pred]
		if !ok || r.Status != StatusDone {
			return false
		}
	}
	return true
}

func (s *Scheduler) setResult(nodeID string, r *TaskResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[nodeID] = r
}

func (s *Scheduler) markRemaining(fromNode string, status TaskStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for _, n := range s.order {
		if n == fromNode {
			found = true
		}
		if found {
			if _, exists := s.results[n]; !exists {
				s.results[n] = &TaskResult{
					NodeID:    n,
					Status:    status,
					StartedAt: time.Now(),
					EndedAt:   time.Now(),
				}
			}
		}
	}
}

func (s *Scheduler) copyResults() map[string]*TaskResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make(map[string]*TaskResult, len(s.results))
	for k, v := range s.results {
		cp[k] = v
	}
	return cp
}

// Summary returns counts by status.
func Summary(results map[string]*TaskResult) map[TaskStatus]int {
	counts := map[TaskStatus]int{}
	for _, r := range results {
		counts[r.Status]++
	}
	return counts
}

// FailedNodes returns IDs of nodes that failed.
func FailedNodes(results map[string]*TaskResult) []string {
	var out []string
	for id, r := range results {
		if r.Status == StatusFailed {
			out = append(out, id)
		}
	}
	return out
}

// CompletedNodes returns IDs of nodes that completed successfully.
func CompletedNodes(results map[string]*TaskResult) []string {
	var out []string
	for id, r := range results {
		if r.Status == StatusDone {
			out = append(out, id)
		}
	}
	return out
}
