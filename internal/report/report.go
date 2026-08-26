package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"etl-lineage/internal/graph"
	"etl-lineage/internal/lineage"
	"etl-lineage/internal/validate"
)

func WriteDOT(w io.Writer, g *graph.Graph) (err error) {
	bw := bufio.NewWriter(w)
	defer func() {
		if ferr := bw.Flush(); ferr != nil && err == nil {
			err = ferr
		}
	}()
	fmt.Fprintln(bw, "digraph lineage {")
	fmt.Fprintln(bw, "  rankdir=LR;")
	fmt.Fprintln(bw, "  node [shape=box];")

	for _, n := range g.Nodes() {
		attr, _ := g.GetNodeAttr(n)
		label := n
		if attr.Layer != "" {
			label = fmt.Sprintf("%s\\n[%s]", n, attr.Layer)
		}
		fmt.Fprintf(bw, "  %q [label=%q];\n", n, label)
	}
	for _, e := range g.Edges() {
		label := string(e.Attr.Transform)
		if label == "" || label == "direct" {
			fmt.Fprintf(bw, "  %q -> %q;\n", e.From, e.To)
		} else {
			fmt.Fprintf(bw, "  %q -> %q [label=%q];\n", e.From, e.To, label)
		}
	}
	fmt.Fprintln(bw, "}")
	return nil
}

type JSONReport struct {
	GeneratedAt string          `json:"generated_at"`
	NodeCount   int             `json:"node_count"`
	EdgeCount   int             `json:"edge_count"`
	Roots       []string        `json:"roots"`
	Leaves      []string        `json:"leaves"`
	Layers      [][]string      `json:"layers"`
	Nodes       []JSONNode      `json:"nodes"`
	Edges       []JSONEdge      `json:"edges"`
	Validation  *JSONValidation `json:"validation,omitempty"`
}

type JSONNode struct {
	ID     string `json:"id"`
	Layer  string `json:"layer,omitempty"`
	Owner  string `json:"owner,omitempty"`
	InDeg  int    `json:"in_degree"`
	OutDeg int    `json:"out_degree"`
}

type JSONEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Transform string `json:"transform"`
	Comment   string `json:"comment,omitempty"`
}

type JSONValidation struct {
	OK       bool     `json:"ok"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func WriteJSON(w io.Writer, g *graph.Graph) error {
	report := buildJSONReport(g)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func buildJSONReport(g *graph.Graph) JSONReport {
	layers, _ := lineage.LayerOrder(g)
	vr := validate.Validate(g)

	report := JSONReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		NodeCount:   g.NodeCount(),
		EdgeCount:   g.EdgeCount(),
		Roots:       g.Roots(),
		Leaves:      g.Leaves(),
		Layers:      layers,
	}

	for _, id := range g.Nodes() {
		attr, _ := g.GetNodeAttr(id)
		report.Nodes = append(report.Nodes, JSONNode{
			ID:     id,
			Layer:  attr.Layer,
			Owner:  attr.Owner,
			InDeg:  len(g.Predecessors(id)),
			OutDeg: len(g.Successors(id)),
		})
	}

	for _, e := range g.Edges() {
		report.Edges = append(report.Edges, JSONEdge{
			From:      e.From,
			To:        e.To,
			Transform: string(e.Attr.Transform),
			Comment:   e.Attr.Comment,
		})
	}

	jv := &JSONValidation{OK: vr.OK()}
	for _, iss := range vr.Errors() {
		jv.Errors = append(jv.Errors, iss.Message)
	}
	for _, iss := range vr.Warnings() {
		jv.Warnings = append(jv.Warnings, iss.Message)
	}
	report.Validation = jv

	return report
}

type ImpactSummary struct {
	ChangedNodes  []string       `json:"changed_nodes"`
	ImpactedNodes []string       `json:"impacted_nodes"`
	ImpactedCount int            `json:"impacted_count"`
	MaxDistance   int            `json:"max_distance"`
	ImpactByLayer map[string]int `json:"impact_by_layer"`
	CriticalPaths [][]string     `json:"critical_paths,omitempty"`
}

func WriteImpactSummary(w io.Writer, g *graph.Graph, changed []string) error {
	summary, err := ComputeImpactSummary(g, changed)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(summary)
}

func ComputeImpactSummary(g *graph.Graph, changed []string) (*ImpactSummary, error) {
	impacted, err := lineage.BatchImpact(g, changed)
	if err != nil {
		return nil, err
	}

	summary := &ImpactSummary{
		ChangedNodes:  changed,
		ImpactedNodes: impacted,
		ImpactedCount: len(impacted),
		ImpactByLayer: map[string]int{},
	}

	maxDist := 0
	for _, c := range changed {
		for _, imp := range impacted {
			d, err := lineage.Distance(g, c, imp)
			if err == nil && d > maxDist {
				maxDist = d
			}
		}
	}
	summary.MaxDistance = maxDist

	for _, n := range impacted {
		attr, err := g.GetNodeAttr(n)
		if err != nil {
			continue
		}
		layer := attr.Layer
		if layer == "" {
			layer = "unknown"
		}
		summary.ImpactByLayer[layer]++
	}

	leaves := g.Leaves()
	impactedSet := map[string]bool{}
	for _, n := range impacted {
		impactedSet[n] = true
	}
	for _, c := range changed {
		for _, leaf := range leaves {
			if !impactedSet[leaf] {
				continue
			}
			paths, err := lineage.AllPaths(g, c, leaf, 1)
			if err == nil && len(paths) > 0 {
				summary.CriticalPaths = append(summary.CriticalPaths, paths[0])
			}
		}
	}

	return summary, nil
}

func WriteTextSummary(w io.Writer, g *graph.Graph) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	fmt.Fprintf(bw, "ETL Lineage Summary\n")
	fmt.Fprintf(bw, "===================\n\n")
	fmt.Fprintf(bw, "Nodes: %d\n", g.NodeCount())
	fmt.Fprintf(bw, "Edges: %d\n", g.EdgeCount())
	fmt.Fprintf(bw, "Roots: %v\n", g.Roots())
	fmt.Fprintf(bw, "Leaves: %v\n\n", g.Leaves())

	layers, err := lineage.LayerOrder(g)
	if err == nil && len(layers) > 0 {
		fmt.Fprintf(bw, "Layer Order:\n")
		for i, layer := range layers {
			fmt.Fprintf(bw, "  L%d: %v\n", i, layer)
		}
		fmt.Fprintln(bw)
	}

	fmt.Fprintf(bw, "Node Details:\n")
	for _, n := range g.Nodes() {
		attr, _ := g.GetNodeAttr(n)
		preds := g.Predecessors(n)
		succs := g.Successors(n)
		fmt.Fprintf(bw, "  %s", n)
		if attr.Layer != "" {
			fmt.Fprintf(bw, " [%s]", attr.Layer)
		}
		if attr.Owner != "" {
			fmt.Fprintf(bw, " owner=%s", attr.Owner)
		}
		fmt.Fprintf(bw, " in=%d out=%d\n", len(preds), len(succs))
	}
	fmt.Fprintln(bw)

	vr := validate.Validate(g)
	if vr.OK() {
		fmt.Fprintf(bw, "Validation: PASS\n")
	} else {
		fmt.Fprintf(bw, "Validation: FAIL\n")
		for _, iss := range vr.Errors() {
			fmt.Fprintf(bw, "  [ERROR] %s\n", iss.Message)
		}
	}
	for _, iss := range vr.Warnings() {
		fmt.Fprintf(bw, "  [WARN] %s\n", iss.Message)
	}

	return nil
}

type DepMatrix struct {
	Nodes  []string `json:"nodes"`
	Matrix [][]bool `json:"matrix"`
}

func BuildDepMatrix(g *graph.Graph) *DepMatrix {
	nodes := g.Nodes()
	idx := map[string]int{}
	for i, n := range nodes {
		idx[n] = i
	}
	matrix := make([][]bool, len(nodes))
	for i := range matrix {
		matrix[i] = make([]bool, len(nodes))
	}
	for _, e := range g.Edges() {
		fi, ok1 := idx[e.From]
		ti, ok2 := idx[e.To]
		if ok1 && ok2 {
			matrix[fi][ti] = true
		}
	}
	return &DepMatrix{Nodes: nodes, Matrix: matrix}
}

func WriteDepMatrix(w io.Writer, g *graph.Graph) error {
	dm := BuildDepMatrix(g)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(dm)
}

type Stats struct {
	NodeCount    int            `json:"node_count"`
	EdgeCount    int            `json:"edge_count"`
	AvgInDegree  float64        `json:"avg_in_degree"`
	AvgOutDegree float64        `json:"avg_out_degree"`
	MaxInDegree  int            `json:"max_in_degree"`
	MaxOutDegree int            `json:"max_out_degree"`
	MaxInNode    string         `json:"max_in_node"`
	MaxOutNode   string         `json:"max_out_node"`
	Depth        int            `json:"depth"`
	LayerCounts  map[string]int `json:"layer_counts"`
}

func ComputeStats(g *graph.Graph) *Stats {
	s := &Stats{
		NodeCount:   g.NodeCount(),
		EdgeCount:   g.EdgeCount(),
		LayerCounts: map[string]int{},
	}
	if s.NodeCount == 0 {
		return s
	}

	totalIn := 0
	totalOut := 0
	for _, n := range g.Nodes() {
		inDeg := len(g.Predecessors(n))
		outDeg := len(g.Successors(n))
		totalIn += inDeg
		totalOut += outDeg
		if inDeg > s.MaxInDegree {
			s.MaxInDegree = inDeg
			s.MaxInNode = n
		}
		if outDeg > s.MaxOutDegree {
			s.MaxOutDegree = outDeg
			s.MaxOutNode = n
		}
		attr, _ := g.GetNodeAttr(n)
		layer := attr.Layer
		if layer == "" {
			layer = "unset"
		}
		s.LayerCounts[layer]++
	}
	s.AvgInDegree = float64(totalIn) / float64(s.NodeCount)
	s.AvgOutDegree = float64(totalOut) / float64(s.NodeCount)

	layers, err := lineage.LayerOrder(g)
	if err == nil {
		s.Depth = len(layers)
	}

	return s
}

func WriteStats(w io.Writer, g *graph.Graph) error {
	s := ComputeStats(g)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

type OwnerReport struct {
	Owners map[string][]string `json:"owners"`
}

func BuildOwnerReport(g *graph.Graph) *OwnerReport {
	r := &OwnerReport{Owners: map[string][]string{}}
	for _, n := range g.Nodes() {
		attr, _ := g.GetNodeAttr(n)
		owner := attr.Owner
		if owner == "" {
			owner = "unassigned"
		}
		r.Owners[owner] = append(r.Owners[owner], n)
	}
	for k := range r.Owners {
		sort.Strings(r.Owners[k])
	}
	return r
}
