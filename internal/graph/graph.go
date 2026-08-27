package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

type TransformType string

const (
	TransformDirect    TransformType = "direct"
	TransformFilter    TransformType = "filter"
	TransformAggregate TransformType = "aggregate"
	TransformJoin      TransformType = "join"
	TransformLookup    TransformType = "lookup"
	TransformUnion     TransformType = "union"
)

var ValidTransformTypes = []TransformType{
	TransformDirect, TransformFilter, TransformAggregate,
	TransformJoin, TransformLookup, TransformUnion,
}

func IsValidTransform(t TransformType) bool {
	for _, v := range ValidTransformTypes {
		if v == t {
			return true
		}
	}
	return false
}

type EdgeAttr struct {
	Transform TransformType `json:"transform"`
	Comment   string        `json:"comment,omitempty"`
}

type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Attr EdgeAttr `json:"attr"`
}

type NodeAttr struct {
	Layer string `json:"layer,omitempty"`
	Owner string `json:"owner,omitempty"`
}

type Graph struct {
	nodes     map[string]*NodeAttr
	edges     map[string]map[string]*EdgeAttr
	nodeOrder []string
}

func New() *Graph {
	return &Graph{
		nodes: map[string]*NodeAttr{},
		edges: map[string]map[string]*EdgeAttr{},
	}
}

func (g *Graph) AddNode(id string) {
	if _, ok := g.nodes[id]; ok {
		return
	}
	g.nodes[id] = &NodeAttr{}
	g.nodeOrder = append(g.nodeOrder, id)
}

func (g *Graph) AddNodeWithAttr(id string, attr NodeAttr) {
	if _, ok := g.nodes[id]; ok {
		g.nodes[id] = &attr
		return
	}
	g.nodes[id] = &attr
	g.nodeOrder = append(g.nodeOrder, id)
}

func (g *Graph) SetNodeAttr(id string, attr NodeAttr) error {
	if !g.HasNode(id) {
		return fmt.Errorf("node %q not found", id)
	}
	g.nodes[id] = &attr
	return nil
}

func (g *Graph) GetNodeAttr(id string) (NodeAttr, error) {
	a, ok := g.nodes[id]
	if !ok {
		return NodeAttr{}, fmt.Errorf("node %q not found", id)
	}
	return *a, nil
}

func (g *Graph) HasNode(id string) bool {
	_, ok := g.nodes[id]
	return ok
}

func (g *Graph) NodeCount() int {
	return len(g.nodes)
}

func (g *Graph) EdgeCount() int {
	count := 0
	for _, tos := range g.edges {
		count += len(tos)
	}
	return count
}

func (g *Graph) Nodes() []string {
	out := make([]string, 0, len(g.nodes))
	for n := range g.nodes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (g *Graph) NodesInOrder() []string {
	out := make([]string, len(g.nodeOrder))
	copy(out, g.nodeOrder)
	return out
}

func (g *Graph) Successors(n string) []string {
	out := make([]string, 0, len(g.edges[n]))
	for to := range g.edges[n] {
		out = append(out, to)
	}
	sort.Strings(out)
	return out
}

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

func (g *Graph) GetEdgeAttr(from, to string) (EdgeAttr, error) {
	if tos, ok := g.edges[from]; ok {
		if attr, ok2 := tos[to]; ok2 {
			return *attr, nil
		}
	}
	return EdgeAttr{}, fmt.Errorf("edge %q->%q not found", from, to)
}

func (g *Graph) HasEdge(from, to string) bool {
	if tos, ok := g.edges[from]; ok {
		_, ok2 := tos[to]
		return ok2
	}
	return false
}

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

func (g *Graph) AddEdge(from, to string) error {
	return g.AddEdgeWithAttr(from, to, EdgeAttr{Transform: TransformDirect})
}

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
		return fmt.Errorf("edge %q->%q would create a cycle", from, to)
	}
	if g.edges[from] == nil {
		g.edges[from] = map[string]*EdgeAttr{}
	}
	g.edges[from][to] = &attr
	return nil
}

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
	for i, n := range g.nodeOrder {
		if n == id {
			g.nodeOrder = append(g.nodeOrder[:i], g.nodeOrder[i+1:]...)
			break
		}
	}
	return nil
}

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

type serialGraph struct {
	Nodes []serialNode `json:"nodes"`
	Edges []Edge       `json:"edges"`
}

type serialNode struct {
	ID   string   `json:"id"`
	Attr NodeAttr `json:"attr"`
}

func (g *Graph) MarshalJSON() ([]byte, error) {
	sg := serialGraph{}
	for _, id := range g.Nodes() {
		sg.Nodes = append(sg.Nodes, serialNode{ID: id, Attr: *g.nodes[id]})
	}
	sg.Edges = g.Edges()
	return json.Marshal(sg)
}

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

func (g *Graph) WriteTo(w io.Writer) (int64, error) {
	data, err := g.MarshalJSON()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

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
