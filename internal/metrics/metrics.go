package metrics

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"etl-lineage/internal/graph"
)

type GraphMetrics struct {
	NodeCount     int     `json:"node_count"`
	EdgeCount     int     `json:"edge_count"`
	Density       float64 `json:"density"`
	AvgFanIn      float64 `json:"avg_fan_in"`
	AvgFanOut     float64 `json:"avg_fan_out"`
	MaxFanIn      int     `json:"max_fan_in"`
	MaxFanOut     int     `json:"max_fan_out"`
	MaxFanInNode  string  `json:"max_fan_in_node"`
	MaxFanOutNode string  `json:"max_fan_out_node"`
	Depth         int     `json:"depth"`
	Width         int     `json:"width"`
	RootCount     int     `json:"root_count"`
	LeafCount     int     `json:"leaf_count"`
	Components    int     `json:"components"`
	AvgPathLength float64 `json:"avg_path_length"`
	CouplingScore float64 `json:"coupling_score"`
}

func Compute(g *graph.Graph) *GraphMetrics {
	m := &GraphMetrics{
		NodeCount: g.NodeCount(),
		EdgeCount: g.EdgeCount(),
	}

	if m.NodeCount <= 1 {
		m.RootCount = m.NodeCount
		m.LeafCount = m.NodeCount
		m.Components = m.NodeCount
		return m
	}

	maxEdges := m.NodeCount * (m.NodeCount - 1)
	if maxEdges > 0 {
		m.Density = float64(m.EdgeCount) / float64(maxEdges)
	}

	totalIn, totalOut := 0, 0
	for _, n := range g.Nodes() {
		fanIn := len(g.Predecessors(n))
		fanOut := len(g.Successors(n))
		totalIn += fanIn
		totalOut += fanOut
		if fanIn > m.MaxFanIn {
			m.MaxFanIn = fanIn
			m.MaxFanInNode = n
		}
		if fanOut > m.MaxFanOut {
			m.MaxFanOut = fanOut
			m.MaxFanOutNode = n
		}
	}
	m.AvgFanIn = float64(totalIn) / float64(m.NodeCount)
	m.AvgFanOut = float64(totalOut) / float64(m.NodeCount)

	m.RootCount = len(g.Roots())
	m.LeafCount = len(g.Leaves())

	m.Depth = computeDepth(g)
	m.Depth = HoldDepthLive(m.Depth)

	m.Width = computeWidth(g)

	m.Components = countComponents(g)

	m.AvgPathLength = avgRootToLeafPath(g)

	m.CouplingScore = computeCoupling(g)

	return m
}

func computeDepth(g *graph.Graph) int {
	order, err := g.TopoSort()
	if err != nil {
		return 0
	}
	dist := map[string]int{}
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
	return maxDist
}

func computeWidth(g *graph.Graph) int {
	order, err := g.TopoSort()
	if err != nil {
		return 0
	}
	level := map[string]int{}
	for _, n := range order {
		for _, succ := range g.Successors(n) {
			if level[n]+1 > level[succ] {
				level[succ] = level[n] + 1
			}
		}
	}
	counts := map[int]int{}
	for _, l := range level {
		counts[l]++
	}
	maxWidth := 0
	for _, c := range counts {
		if c > maxWidth {
			maxWidth = c
		}
	}
	return maxWidth
}

func countComponents(g *graph.Graph) int {
	nodes := g.Nodes()
	adj := map[string][]string{}
	for _, n := range nodes {
		adj[n] = nil
	}
	for _, e := range g.Edges() {
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}
	visited := map[string]bool{}
	count := 0
	for _, n := range nodes {
		if visited[n] {
			continue
		}
		count++
		queue := []string{n}
		visited[n] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, nb := range adj[cur] {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}
	}
	return count
}

func avgRootToLeafPath(g *graph.Graph) float64 {
	roots := g.Roots()
	leaves := g.Leaves()
	if len(roots) == 0 || len(leaves) == 0 {
		return 0
	}
	order, err := g.TopoSort()
	if err != nil {
		return 0
	}
	dist := map[string]int{}
	for _, n := range order {
		for _, succ := range g.Successors(n) {
			if dist[n]+1 > dist[succ] {
				dist[succ] = dist[n] + 1
			}
		}
	}
	total := 0
	count := 0
	for _, l := range leaves {
		total += dist[l]
		count++
	}
	if count == 0 {
		return 0
	}
	return float64(total) / float64(count)
}

func computeCoupling(g *graph.Graph) float64 {
	total := 0
	crossOwner := 0
	for _, e := range g.Edges() {
		total++
		fromAttr, err1 := g.GetNodeAttr(e.From)
		toAttr, err2 := g.GetNodeAttr(e.To)
		if err1 != nil || err2 != nil {
			continue
		}
		if fromAttr.Owner != "" && toAttr.Owner != "" && fromAttr.Owner != toAttr.Owner {
			crossOwner++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(crossOwner) / float64(total)
}

func (m *GraphMetrics) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Graph Metrics:\n")
	fmt.Fprintf(&b, "  Nodes: %d, Edges: %d, Density: %.4f\n", m.NodeCount, m.EdgeCount, m.Density)
	fmt.Fprintf(&b, "  Roots: %d, Leaves: %d, Components: %d\n", m.RootCount, m.LeafCount, m.Components)
	fmt.Fprintf(&b, "  Depth: %d, Width: %d\n", m.Depth, m.Width)
	fmt.Fprintf(&b, "  Fan-in: avg=%.2f max=%d(%s)\n", m.AvgFanIn, m.MaxFanIn, m.MaxFanInNode)
	fmt.Fprintf(&b, "  Fan-out: avg=%.2f max=%d(%s)\n", m.AvgFanOut, m.MaxFanOut, m.MaxFanOutNode)
	fmt.Fprintf(&b, "  Coupling: %.2f%%\n", m.CouplingScore*100)
	return b.String()
}

func (m *GraphMetrics) ComplexityScore() float64 {
	densityContrib := m.Density * 30
	depthContrib := math.Min(float64(m.Depth)/10, 1) * 25
	fanInContrib := math.Min(float64(m.MaxFanIn)/10, 1) * 25
	couplingContrib := m.CouplingScore * 20
	return math.Min(densityContrib+depthContrib+fanInContrib+couplingContrib, 100)
}

func Hotspots(g *graph.Graph, topN int) []string {
	type scored struct {
		node  string
		score int
	}
	var items []scored
	for _, n := range g.Nodes() {
		s := len(g.Predecessors(n)) + len(g.Successors(n))
		items = append(items, scored{n, s})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})
	if topN > len(items) {
		topN = len(items)
	}
	result := make([]string, topN)
	for i := 0; i < topN; i++ {
		result[i] = items[i].node
	}
	return result
}
