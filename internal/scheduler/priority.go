package scheduler

import (
	"sort"

	"etl-lineage/internal/graph"
)

// PriorityStrategy defines how to order nodes within the same topological level.
type PriorityStrategy int

const (
	// PriorityCriticalPath prioritizes nodes on the longest path first.
	PriorityCriticalPath PriorityStrategy = iota

	// PriorityFanOut prioritizes nodes with more downstream dependents first.
	PriorityFanOut

	// PriorityFanIn prioritizes nodes with more upstream dependencies first.
	PriorityFanIn

	// PriorityAlphabetic uses alphabetical order (default/deterministic).
	PriorityAlphabetic
)

// PrioritizedOrder returns a topological ordering that respects the given
// priority strategy for tie-breaking within the same level.
func PrioritizedOrder(g *graph.Graph, strategy PriorityStrategy) ([]string, error) {
	order, err := g.TopoSort()
	if err != nil {
		return nil, err
	}

	// compute levels (distance from roots)
	level := computeLevels(g, order)

	// group by level
	maxLevel := 0
	for _, l := range level {
		if l > maxLevel {
			maxLevel = l
		}
	}
	groups := make([][]string, maxLevel+1)
	for _, n := range order {
		groups[level[n]] = append(groups[level[n]], n)
	}

	// sort each group by priority
	for i := range groups {
		sortByStrategy(groups[i], g, strategy)
	}

	// flatten
	result := make([]string, 0, len(order))
	for _, grp := range groups {
		result = append(result, grp...)
	}
	return result, nil
}

func computeLevels(g *graph.Graph, order []string) map[string]int {
	level := make(map[string]int, len(order))
	for _, n := range order {
		level[n] = 0
		for _, pred := range g.Predecessors(n) {
			if level[pred]+1 > level[n] {
				level[n] = level[pred] + 1
			}
		}
	}
	return level
}

func sortByStrategy(nodes []string, g *graph.Graph, strategy PriorityStrategy) {
	switch strategy {
	case PriorityCriticalPath:
		// critical path: prefer nodes with longer downstream path
		sort.Slice(nodes, func(i, j int) bool {
			di := downstreamDepth(g, nodes[i])
			dj := downstreamDepth(g, nodes[j])
			if di != dj {
				return di > dj
			}
			return nodes[i] < nodes[j]
		})
	case PriorityFanOut:
		sort.Slice(nodes, func(i, j int) bool {
			fi := fanOut(g, nodes[i])
			fj := fanOut(g, nodes[j])
			if fi != fj {
				return fi > fj
			}
			return nodes[i] < nodes[j]
		})
	case PriorityFanIn:
		sort.Slice(nodes, func(i, j int) bool {
			fi := len(g.Predecessors(nodes[i]))
			fj := len(g.Predecessors(nodes[j]))
			if fi != fj {
				return fi > fj
			}
			return nodes[i] < nodes[j]
		})
	default:
		sort.Strings(nodes)
	}
}

// downstreamDepth returns the longest directed path length from node to any leaf.
func downstreamDepth(g *graph.Graph, node string) int {
	best := 0
	var dfs func(n string, depth int)
	dfs = func(n string, depth int) {
		succs := g.Successors(n)
		if len(succs) == 0 {
			if depth > best {
				best = depth
			}
			return
		}
		for _, s := range succs {
			dfs(s, depth+1)
		}
	}
	dfs(node, 0)
	return best
}

// fanOut returns total transitive downstream count.
func fanOut(g *graph.Graph, node string) int {
	visited := map[string]bool{}
	var dfs func(string)
	dfs = func(n string) {
		for _, s := range g.Successors(n) {
			if !visited[s] {
				visited[s] = true
				dfs(s)
			}
		}
	}
	dfs(node)
	return len(visited)
}

// EstimateCriticalPathLength returns the length of the longest path in the DAG.
func EstimateCriticalPathLength(g *graph.Graph) (int, error) {
	order, err := g.TopoSort()
	if err != nil {
		return 0, err
	}
	dist := make(map[string]int, len(order))
	maxDist := 0
	for _, n := range order {
		for _, succ := range g.Successors(n) {
			if dist[n]+1 > dist[succ] {
				dist[succ] = dist[n] + 1
			}
		}
		if dist[n] > maxDist {
			maxDist = dist[n]
		}
	}
	return maxDist, nil
}
