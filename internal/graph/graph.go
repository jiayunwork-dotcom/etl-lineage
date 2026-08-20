package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

// TransformType describes how a source contributes to a target.
type TransformType string

const (
	TransformDirect    TransformType = "direct"
	TransformFilter    TransformType = "filter"
	TransformAggregate TransformType = "aggregate"
	TransformJoin      TransformType = "join"
	TransformLookup    TransformType = "lookup"
	TransformUnion     TransformType = "union"
)

// ValidTransformTypes lists all recognized transform types.
var ValidTransformTypes = []TransformType{
	TransformDirect, TransformFilter, TransformAggregate,
	TransformJoin, TransformLookup, TransformUnion,
}

// IsValidTransform returns true if t is a recognized transform type.
func IsValidTransform(t TransformType) bool {
	for _, v := range ValidTransformTypes {
		if v == t {
			return true
		}
	}
	return false
}

// EdgeAttr holds metadata for a directed edge.
type EdgeAttr struct {
	Transform TransformType `json:"transform"`
	Comment   string        `json:"comment,omitempty"`
}

// Edge represents a directed edge with attributes.
type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Attr EdgeAttr `json:"attr"`
}

// NodeAttr holds optional metadata for a node.
type NodeAttr struct {
	Layer string `json:"layer,omitempty"` // e.g. "staging", "dim", "fact", "report"
	Owner string `json:"owner,omitempty"`
}

// Graph is a directed acyclic graph with attributed nodes and edges.
type Graph struct {
	nodes     map[string]*NodeAttr
	edges     map[string]map[string]*EdgeAttr // from -> to -> attr
	nodeOrder []string                        // insertion order for determinism
}

// New creates an empty graph.
func New() *Graph {
	return &Graph{
		nodes: map[string]*NodeAttr{},
		edges: map[string]map[string]*EdgeAttr{},
	}
}

// AddNode adds a node. If it already exists, this is a no-op.
func (g *Graph) AddNode(id string) {
	if _, ok := g.nodes[id]; ok {
		return
	}
	g.nodes[id] = &NodeAttr{}
	g.nodeOrder = append(g.nodeOrder, id)
}

// AddNodeWithAttr adds a node with attributes.
func (g *Graph) AddNodeWithAttr(id string, attr NodeAttr) {
	if _, ok := g.nodes[id]; ok {
		g.nodes[id] = &attr
		return
	}
	g.nodes[id] = &attr
	g.nodeOrder = append(g.nodeOrder, id)
}

// SetNodeAttr updates the attributes of an existing node.
func (g *Graph) SetNodeAttr(id string, attr NodeAttr) error {
	if !g.HasNode(id) {
		return fmt.Errorf("node %q not found", id)
	}
	g.nodes[id] = &attr
	return nil
}

// GetNodeAttr returns attributes for a node.
func (g *Graph) GetNodeAttr(id string) (NodeAttr, error) {
	a, ok := g.nodes[id]
	if !ok {
		return NodeAttr{}, fmt.Errorf("node %q not found", id)
	}
	return *a, nil
}

// HasNode checks if a node exists.
func (g *Graph) HasNode(id string) bool {
	_, ok := g.nodes[id]
	return ok
}

// NodeCount returns the number of nodes.
func (g *Graph) NodeCount() int {
	return len(g.nodes)
}

// EdgeCount returns the total number of edges.
func (g *Graph) EdgeCount() int {
	count := 0
	for _, tos := range g.edges {
		count += len(tos)
	}
	return count
}

// Nodes returns all node IDs in sorted order.
func (g *Graph) Nodes() []string {
	out := make([]string, 0, len(g.nodes))
	for n := range g.nodes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// NodesInOrder returns node IDs in insertion order.
func (g *Graph) NodesInOrder() []string {
	out := make([]string, len(g.nodeOrder))
	copy(out, g.nodeOrder)
	return out
}

// Successors returns sorted direct successors of n.
func (g *Graph) Successors(n string) []string {
	out := make([]string, 0, len(g.edges[n]))
	for to := range g.edges[n] {
		out = append(out, to)
	}
	sort.Strings(out)
	return out
}

// Predecessors returns sorted direct predecessors of n.
func (g *Graph) Predecessors(n string) []string {
	var out []string
	for from, tos := range g.edges {
		if _, ok := tos[n]; ok {
			out = append(out, from)
		}
	}
	sort.Strings(out)
	return out
}

// Edges returns all edges in deterministic order.
func (g *Graph) Edges() []Edge {
	var out []Edge
	froms := make([]string, 0, len(g.edges))
	for f := range g.edges {
		froms = append(froms, f)
	}
	sort.Strings(froms)
	for _, f := range froms {
		tos := make([]string, 0, len(g.edges[f]))
		for t := range g.edges[f] {
			tos = append(tos, t)
		}
		sort.Strings(tos)
		for _, t := range tos {
			out = append(out, Edge{From: f, To: t, Attr: *g.edges[f][t]})
		}
	}
	return out
}

// GetEdgeAttr returns edge attributes, or error if edge does not exist.
func (g *Graph) GetEdgeAttr(from, to string) (EdgeAttr, error) {
	if tos, ok := g.edges[from]; ok {
		if attr, ok2 := tos[to]; ok2 {
			return *attr, nil
		}
	}
	return EdgeAttr{}, fmt.Errorf("edge %q->%q not found", from, to)
}

// HasEdge returns true if the edge from->to exists.
func (g *Graph) HasEdge(from, to string) bool {
	if tos, ok := g.edges[from]; ok {
		_, ok2 := tos[to]
		return ok2
	}
	return false
}

// reaches checks if target is reachable from start via DFS.
func (g *Graph) reaches(from, target string) bool {
	seen := map[string]bool{}
	stack := []string{from}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == target {
			return true
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		for to := range g.edges[n] {
			stack = append(stack, to)
		}
	}
	return false
}

// AddEdge adds an edge from->to with default attributes. Returns error on
// unknown nodes, self-loops, or cycles.
func (g *Graph) AddEdge(from, to string) error {
	return g.AddEdgeWithAttr(from, to, EdgeAttr{Transform: TransformDirect})
}

// AddEdgeWithAttr adds an edge with explicit attributes.
func (g *Graph) AddEdgeWithAttr(from, to string, attr EdgeAttr) error {
	if !g.HasNode(from) {
		return fmt.Errorf("edge from unknown node %q", from)
	}
	if !g.HasNode(to) {
		return fmt.Errorf("edge to unknown node %q", to)
	}
	if from == to {
		return errors.New("self-loop not allowed")
	}
	if g.reaches(to, from) {
		return commitCycle(fmt.Errorf("edge %q->%q would create a cycle", from, to))
	}
	if g.edges[from] == nil {
		g.edges[from] = map[string]*EdgeAttr{}
	}
	g.edges[from][to] = &attr
	return nil
}

// RemoveEdge removes an edge. Returns error if edge does not exist.
func (g *Graph) RemoveEdge(from, to string) error {
	if tos, ok := g.edges[from]; ok {
		if _, ok2 := tos[to]; ok2 {
			delete(tos, to)
			if len(tos) == 0 {
				delete(g.edges, from)
			}
			return nil
		}
	}
	return fmt.Errorf("edge %q->%q not found", from, to)
}

// RemoveNode removes a node and all its incident edges.
func (g *Graph) RemoveNode(id string) error {
	if !g.HasNode(id) {
		return fmt.Errorf("node %q not found", id)
	}
	delete(g.nodes, id)
	delete(g.edges, id)
	for from, tos := range g.edges {
		delete(tos, id)
		if len(tos) == 0 {
			delete(g.edges, from)
		}
	}
	// remove from insertion order
	for i, n := range g.nodeOrder {
		if n == id {
			g.nodeOrder = append(g.nodeOrder[:i], g.nodeOrder[i+1:]...)
			break
		}
	}
	return nil
}

// SubGraph extracts a subgraph containing only the given nodes and edges between them.
func (g *Graph) SubGraph(nodeIDs []string) *Graph {
	sub := New()
	keep := map[string]bool{}
	for _, id := range nodeIDs {
		if g.HasNode(id) {
			keep[id] = true
			attr := *g.nodes[id]
			sub.AddNodeWithAttr(id, attr)
		}
	}
	for from, tos := range g.edges {
		if !keep[from] {
			continue
		}
		for to, attr := range tos {
			if !keep[to] {
				continue
			}
			sub.AddEdgeWithAttr(from, to, *attr)
		}
	}
	return sub
}

// TopoSort returns a topological ordering of nodes. Returns error if graph has a cycle.
func (g *Graph) TopoSort() ([]string, error) {
	indeg := map[string]int{}
	for n := range g.nodes {
		indeg[n] = 0
	}
	for from := range g.edges {
		for to := range g.edges[from] {
			indeg[to]++
		}
	}
	visited := map[string]bool{}
	order := make([]string, 0, len(g.nodes))
	for len(visited) < len(g.nodes) {
		var ready []string
		for n := range g.nodes {
			if !visited[n] && indeg[n] == 0 {
				ready = append(ready, n)
			}
		}
		if len(ready) == 0 {
			return nil, errors.New("graph has a cycle")
		}
		sort.Strings(ready)
		n := ready[0]
		visited[n] = true
		order = append(order, n)
		for to := range g.edges[n] {
			indeg[to]--
		}
	}
	return order, nil
}

// Roots returns nodes with no incoming edges (in-degree 0).
func (g *Graph) Roots() []string {
	indeg := map[string]int{}
	for n := range g.nodes {
		indeg[n] = 0
	}
	for _, tos := range g.edges {
		for to := range tos {
			indeg[to]++
		}
	}
	var roots []string
	for n, d := range indeg {
		if d == 0 {
			roots = append(roots, n)
		}
	}
	sort.Strings(roots)
	return roots
}

// Leaves returns nodes with no outgoing edges (out-degree 0).
func (g *Graph) Leaves() []string {
	var leaves []string
	for n := range g.nodes {
		if len(g.edges[n]) == 0 {
			leaves = append(leaves, n)
		}
	}
	sort.Strings(leaves)
	return leaves
}

// --- Serialization ---

type serialGraph struct {
	Nodes []serialNode `json:"nodes"`
	Edges []Edge       `json:"edges"`
}

type serialNode struct {
	ID   string   `json:"id"`
	Attr NodeAttr `json:"attr"`
}

// MarshalJSON serializes the graph to JSON.
func (g *Graph) MarshalJSON() ([]byte, error) {
	sg := serialGraph{}
	for _, id := range g.Nodes() {
		sg.Nodes = append(sg.Nodes, serialNode{ID: id, Attr: *g.nodes[id]})
	}
	sg.Edges = g.Edges()
	return json.Marshal(sg)
}

// UnmarshalJSON deserializes a graph from JSON.
func (g *Graph) UnmarshalJSON(data []byte) error {
	var sg serialGraph
	if err := json.Unmarshal(data, &sg); err != nil {
		return err
	}
	g.nodes = map[string]*NodeAttr{}
	g.edges = map[string]map[string]*EdgeAttr{}
	g.nodeOrder = nil
	for _, sn := range sg.Nodes {
		attr := sn.Attr
		g.nodes[sn.ID] = &attr
		g.nodeOrder = append(g.nodeOrder, sn.ID)
	}
	for _, e := range sg.Edges {
		if !g.HasNode(e.From) || !g.HasNode(e.To) {
			return fmt.Errorf("edge references unknown node: %q->%q", e.From, e.To)
		}
		if g.edges[e.From] == nil {
			g.edges[e.From] = map[string]*EdgeAttr{}
		}
		attr := e.Attr
		g.edges[e.From][e.To] = &attr
	}
	return nil
}

// WriteTo writes JSON representation to w.
func (g *Graph) WriteTo(w io.Writer) (int64, error) {
	data, err := g.MarshalJSON()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// ReadFrom reads JSON representation from r and replaces current state.
func (g *Graph) ReadFrom(r io.Reader) (int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	if err := g.UnmarshalJSON(data); err != nil {
		return int64(len(data)), err
	}
	return int64(len(data)), nil
}

// Clone creates a deep copy of the graph.
func (g *Graph) Clone() *Graph {
	c := New()
	for _, id := range g.nodeOrder {
		attr := *g.nodes[id]
		c.AddNodeWithAttr(id, attr)
	}
	for from, tos := range g.edges {
		for to, attr := range tos {
			a := *attr
			c.AddEdgeWithAttr(from, to, a)
		}
	}
	return c
}
