package validate

import (
	"fmt"
	"sort"

	"etl-lineage/internal/graph"
)

type Issue struct {
	Severity string
	Code     string
	Message  string
	Node     string
	Edge     [2]string
}

type Result struct {
	Issues []Issue
}

func (r *Result) OK() bool {
	for _, iss := range r.Issues {
		if iss.Severity == "error" {
			return false
		}
	}
	return true
}

func (r *Result) Errors() []Issue {
	var out []Issue
	for _, iss := range r.Issues {
		if iss.Severity == "error" {
			out = append(out, iss)
		}
	}
	return out
}

func (r *Result) Warnings() []Issue {
	var out []Issue
	for _, iss := range r.Issues {
		if iss.Severity == "warning" {
			out = append(out, iss)
		}
	}
	return out
}

func Validate(g *graph.Graph) *Result {
	r := &Result{}
	checkOrphans(g, r)
	checkDAGIntegrity(g, r)
	checkEdgeTransforms(g, r)
	checkLayerConsistency(g, r)
	checkConnectivity(g, r)
	return r
}

func checkOrphans(g *graph.Graph, r *Result) {
	for _, n := range g.Nodes() {
		preds := g.Predecessors(n)
		succs := g.Successors(n)
		if len(preds) == 0 && len(succs) == 0 && g.NodeCount() > 1 {
			r.Issues = append(r.Issues, Issue{
				Severity: "warning",
				Code:     "orphan_node",
				Message:  fmt.Sprintf("node %q has no edges (orphan)", n),
				Node:     n,
			})
		}
	}
}

func checkDAGIntegrity(g *graph.Graph, r *Result) {
	_, err := g.TopoSort()
	if err != nil {
		r.Issues = append(r.Issues, Issue{
			Severity: "error",
			Code:     "cycle_detected",
			Message:  "graph contains a cycle; DAG invariant violated",
		})
	}
}

func checkEdgeTransforms(g *graph.Graph, r *Result) {
	for _, e := range g.Edges() {
		if e.Attr.Transform == "" {
			r.Issues = append(r.Issues, Issue{
				Severity: "warning",
				Code:     "empty_transform",
				Message:  fmt.Sprintf("edge %q->%q has empty transform type", e.From, e.To),
				Edge:     [2]string{e.From, e.To},
			})
		} else if !graph.IsValidTransform(e.Attr.Transform) {
			r.Issues = append(r.Issues, Issue{
				Severity: "error",
				Code:     "invalid_transform",
				Message:  fmt.Sprintf("edge %q->%q has invalid transform %q", e.From, e.To, e.Attr.Transform),
				Edge:     [2]string{e.From, e.To},
			})
		}
	}
}

func checkLayerConsistency(g *graph.Graph, r *Result) {
	layerRank := map[string]int{
		"staging": 0,
		"dim":     1,
		"fact":    2,
		"report":  3,
	}

	for _, e := range g.Edges() {
		fromAttr, err1 := g.GetNodeAttr(e.From)
		toAttr, err2 := g.GetNodeAttr(e.To)
		if err1 != nil || err2 != nil {
			continue
		}
		if fromAttr.Layer == "" || toAttr.Layer == "" {
			continue
		}
		fromRank, ok1 := layerRank[fromAttr.Layer]
		toRank, ok2 := layerRank[toAttr.Layer]
		if !ok1 || !ok2 {
			continue
		}
		if fromRank > toRank {
			r.Issues = append(r.Issues, Issue{
				Severity: "error",
				Code:     "layer_violation",
				Message: fmt.Sprintf("edge %q(%s)->%q(%s) flows from higher to lower layer",
					e.From, fromAttr.Layer, e.To, toAttr.Layer),
				Edge: [2]string{e.From, e.To},
			})
		}
	}
}

func checkConnectivity(g *graph.Graph, r *Result) {
	nodes := g.Nodes()
	if len(nodes) <= 1 {
		return
	}

	adj := map[string]map[string]bool{}
	for _, n := range nodes {
		adj[n] = map[string]bool{}
	}
	for _, e := range g.Edges() {
		adj[e.From][e.To] = true
		adj[e.To][e.From] = true
	}

	visited := map[string]bool{}
	components := 0

	var bfs func(start string)
	bfs = func(start string) {
		queue := []string{start}
		visited[start] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for nb := range adj[cur] {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}
	}

	for _, n := range nodes {
		if !visited[n] {
			components++
			bfs(n)
		}
	}

	if components > 1 {
		r.Issues = append(r.Issues, Issue{
			Severity: "warning",
			Code:     "disconnected",
			Message:  fmt.Sprintf("graph has %d disconnected components", components),
		})
	}
}

func DetectOrphans(g *graph.Graph) []string {
	var orphans []string
	for _, n := range g.Nodes() {
		if len(g.Predecessors(n)) == 0 && len(g.Successors(n)) == 0 && g.NodeCount() > 1 {
			orphans = append(orphans, n)
		}
	}
	sort.Strings(orphans)
	return orphans
}

func CheckDuplicateEdges(g *graph.Graph) []Issue {
	seen := map[[2]string]int{}
	for _, e := range g.Edges() {
		key := [2]string{e.From, e.To}
		seen[key]++
	}
	var issues []Issue
	for key, count := range seen {
		if count > 1 {
			issues = append(issues, Issue{
				Severity: "error",
				Code:     "duplicate_edge",
				Message:  fmt.Sprintf("edge %q->%q appears %d times", key[0], key[1], count),
				Edge:     key,
			})
		}
	}
	return issues
}

func ValidateNodeNaming(g *graph.Graph) []Issue {
	var issues []Issue
	for _, n := range g.Nodes() {
		for _, ch := range n {
			if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
				issues = append(issues, Issue{
					Severity: "warning",
					Code:     "naming_violation",
					Message:  fmt.Sprintf("node %q contains invalid character %q", n, string(ch)),
					Node:     n,
				})
				break
			}
		}
	}
	return issues
}

func ValidateComplete(g *graph.Graph) *Result {
	r := Validate(g)
	r.Issues = append(r.Issues, CheckDuplicateEdges(g)...)
	r.Issues = append(r.Issues, ValidateNodeNaming(g)...)
	return r
}
