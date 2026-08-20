// Package lineage provides graph traversal operations for ETL lineage analysis:
// upstream/downstream tracing, batch impact analysis, shortest distance, and path enumeration.
package lineage

import (
	"errors"
	"sort"

	"etl-lineage/internal/graph"
)

// Upstream returns all transitive predecessors of node id (excluding id itself).
func Upstream(g *graph.Graph, id string) (map[string]bool, error) {
	if !g.HasNode(id) {
		return nil, errors.New("node not found")
	}
	res := map[string]bool{}
	var dfs func(string)
	dfs = func(n string) {
		for _, from := range g.Predecessors(n) {
			if !res[from] {
				res[from] = true
				dfs(from)
			}
		}
	}
	dfs(id)
	delete(res, id)
	return fillUp(res), nil
}

// Downstream returns all transitive successors of node id (excluding id itself).
func Downstream(g *graph.Graph, id string) (map[string]bool, error) {
	if !g.HasNode(id) {
		return nil, errors.New("node not found")
	}
	res := map[string]bool{}
	var dfs func(string)
	dfs = func(n string) {
		for _, to := range g.Successors(n) {
			if !res[to] {
				res[to] = true
				dfs(to)
			}
		}
	}
	dfs(id)
	delete(res, id)
	return res, nil
}

// BatchImpact computes the union of downstream sets for multiple changed nodes.
// The result is sorted and excludes the changed nodes themselves.
func BatchImpact(g *graph.Graph, changed []string) ([]string, error) {
	if len(changed) == 0 {
		return nil, nil
	}
	changedSet := map[string]bool{}
	for _, c := range changed {
		changedSet[c] = true
	}
	union := map[string]bool{}
	for _, c := range changed {
		down, err := Downstream(g, c)
		if err != nil {
			return nil, err
		}
		for k := range down {
			union[k] = true
		}
	}
	// Exclude the changed nodes themselves from impact
	for _, c := range changed {
		delete(union, c)
	}
	result := make([]string, 0, len(union))
	for k := range union {
		result = append(result, k)
	}
	sort.Strings(result)
	return result, nil
}

// Distance returns the shortest directed distance (number of edges) from src to dst.
// Returns -1 if dst is not reachable from src.
func Distance(g *graph.Graph, src, dst string) (int, error) {
	if !g.HasNode(src) {
		return -1, errors.New("source node not found")
	}
	if !g.HasNode(dst) {
		return -1, errors.New("destination node not found")
	}
	if src == dst {
		return 0, nil
	}
	// BFS
	type item struct {
		node string
		dist int
	}
	visited := map[string]bool{src: true}
	queue := []item{{node: src, dist: 0}}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range g.Successors(cur.node) {
			if next == dst {
				return cur.dist + 1, nil
			}
			if !visited[next] {
				visited[next] = true
				queue = append(queue, item{node: next, dist: cur.dist + 1})
			}
		}
	}
	return -1, nil
}

// AllPaths enumerates all directed paths from src to dst in the DAG.
// Each path is a slice of node IDs including src and dst.
// maxPaths limits the number of paths returned (0 = unlimited).
func AllPaths(g *graph.Graph, src, dst string, maxPaths int) ([][]string, error) {
	if !g.HasNode(src) {
		return nil, errors.New("source node not found")
	}
	if !g.HasNode(dst) {
		return nil, errors.New("destination node not found")
	}
	if src == dst {
		return [][]string{{src}}, nil
	}
	var paths [][]string
	var dfs func(cur string, path []string)
	dfs = func(cur string, path []string) {
		if maxPaths > 0 && len(paths) >= maxPaths {
			return
		}
		if cur == dst {
			p := make([]string, len(path))
			copy(p, path)
			paths = append(paths, p)
			return
		}
		for _, next := range g.Successors(cur) {
			// No cycle in DAG, but guard against revisit in same path
			visited := false
			for _, v := range path {
				if v == next {
					visited = true
					break
				}
			}
			if !visited {
				dfs(next, append(path, next))
			}
		}
	}
	dfs(src, []string{src})
	return paths, nil
}

// LongestPath returns the longest directed path length (in edges) from src to dst.
// Returns -1 if dst is not reachable from src.
func LongestPath(g *graph.Graph, src, dst string) (int, error) {
	if !g.HasNode(src) {
		return -1, errors.New("source node not found")
	}
	if !g.HasNode(dst) {
		return -1, errors.New("destination node not found")
	}
	if src == dst {
		return 0, nil
	}
	best := -1
	var dfs func(cur string, depth int)
	dfs = func(cur string, depth int) {
		if cur == dst {
			if depth > best {
				best = depth
			}
			return
		}
		for _, next := range g.Successors(cur) {
			dfs(next, depth+1)
		}
	}
	dfs(src, 0)
	return best, nil
}

// CriticalPath returns the nodes on the longest path from any root to any leaf in the graph.
// This represents the pipeline's critical execution sequence.
func CriticalPath(g *graph.Graph) ([]string, error) {
	roots := g.Roots()
	leaves := g.Leaves()
	if len(roots) == 0 || len(leaves) == 0 {
		return nil, nil
	}

	var bestPath []string
	for _, r := range roots {
		for _, l := range leaves {
			paths, err := AllPaths(g, r, l, 100)
			if err != nil {
				continue
			}
			for _, p := range paths {
				if len(p) > len(bestPath) {
					bestPath = p
				}
			}
		}
	}
	return bestPath, nil
}

// LayerOrder returns nodes grouped by their topological layer (distance from roots).
// Layer 0 contains roots, layer 1 contains their direct successors, etc.
func LayerOrder(g *graph.Graph) ([][]string, error) {
	topo, err := g.TopoSort()
	if err != nil {
		return nil, err
	}
	if len(topo) == 0 {
		return nil, nil
	}

	dist := map[string]int{}
	for _, n := range topo {
		dist[n] = 0
	}
	for _, n := range topo {
		for _, succ := range g.Successors(n) {
			if dist[n]+1 > dist[succ] {
				dist[succ] = dist[n] + 1
			}
		}
	}

	maxLayer := 0
	for _, d := range dist {
		if d > maxLayer {
			maxLayer = d
		}
	}

	layers := make([][]string, maxLayer+1)
	for n, d := range dist {
		layers[d] = append(layers[d], n)
	}
	for i := range layers {
		sort.Strings(layers[i])
	}
	return layers, nil
}

// CommonAncestors returns nodes that are upstream of all given target nodes.
func CommonAncestors(g *graph.Graph, targets []string) ([]string, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	var common map[string]bool
	for i, t := range targets {
		up, err := Upstream(g, t)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			common = up
		} else {
			for k := range common {
				if !up[k] {
					delete(common, k)
				}
			}
		}
	}
	result := make([]string, 0, len(common))
	for k := range common {
		result = append(result, k)
	}
	sort.Strings(result)
	return result, nil
}
