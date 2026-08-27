package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"etl-lineage/internal/graph"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusDone      TaskStatus = "done"
	StatusFailed    TaskStatus = "failed"
	StatusSkipped   TaskStatus = "skipped"
	StatusCancelled TaskStatus = "cancelled"
)

type TaskResult struct {
	NodeID    string        `json:"node_id"`
	Status    TaskStatus    `json:"status"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
	Duration  time.Duration `json:"duration"`
	Error     error         `json:"-"`
}

type TaskFunc func(ctx context.Context, nodeID string) error

type Scheduler struct {
	mu      sync.Mutex
	g       *graph.Graph
	results map[string]*TaskResult
	order   []string
}

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

func (s *Scheduler) Results() map[string]*TaskResult {
	return s.copyResults()
}

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

func Summary(results map[string]*TaskResult) map[TaskStatus]int {
	counts := map[TaskStatus]int{}
	for _, r := range results {
		counts[r.Status]++
	}
	return counts
}

func FailedNodes(results map[string]*TaskResult) []string {
	var out []string
	for id, r := range results {
		if r.Status == StatusFailed {
			out = append(out, id)
		}
	}
	return out
}

func CompletedNodes(results map[string]*TaskResult) []string {
	var out []string
	for id, r := range results {
		if r.Status == StatusDone {
			out = append(out, id)
		}
	}
	return out
}
