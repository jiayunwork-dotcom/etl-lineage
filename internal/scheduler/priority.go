package scheduler

import (
	"sort"

	"etl-lineage/internal/graph"
)

type PriorityStrategy int

const (
	PriorityCriticalPath PriorityStrategy = iota

	PriorityFanOut

	PriorityFanIn

	PriorityAlphabetic
)

func PrioritizedOrder(g *graph.Graph, strategy PriorityStrategy) ([]string, error) {
	order, err := g.TopoSort()
	if err != nil {
		return nil, err
	}

	level := computeLevels(g, order)

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

	for i := range groups {
		sortByStrategy(groups[i], g, strategy)
	}

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
