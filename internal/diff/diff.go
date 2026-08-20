// Package diff computes structural differences between two lineage graph versions.
// It identifies added/removed/modified nodes and edges, enabling change tracking
// and audit trails across ETL pipeline evolution.
package diff

import (
	"fmt"
	"sort"

	"etl-lineage/internal/graph"
)

// ChangeKind classifies a single change between graph versions.
type ChangeKind string

const (
	ChangeAddNode    ChangeKind = "add_node"
	ChangeRemoveNode ChangeKind = "remove_node"
	ChangeModifyNode ChangeKind = "modify_node"
	ChangeAddEdge    ChangeKind = "add_edge"
	ChangeRemoveEdge ChangeKind = "remove_edge"
	ChangeModifyEdge ChangeKind = "modify_edge"
)

// Change describes a single difference between two graphs.
type Change struct {
	Kind    ChangeKind       `json:"kind"`
	NodeID  string           `json:"node_id,omitempty"`
	From    string           `json:"from,omitempty"`
	To      string           `json:"to,omitempty"`
	OldAttr *graph.NodeAttr  `json:"old_attr,omitempty"`
	NewAttr *graph.NodeAttr  `json:"new_attr,omitempty"`
	OldEdge *graph.EdgeAttr  `json:"old_edge,omitempty"`
	NewEdge *graph.EdgeAttr  `json:"new_edge,omitempty"`
}

// DiffResult holds all changes between two graph versions.
type DiffResult struct {
	Changes  []Change `json:"changes"`
	Added    int      `json:"added"`
	Removed  int      `json:"removed"`
	Modified int      `json:"modified"`
}

// Compare produces a diff between the old and new graph.
func Compare(old, new *graph.Graph) *DiffResult {
	result := &DiffResult{}

	oldNodes := setOf(old.Nodes())
	newNodes := setOf(new.Nodes())

	// Nodes added
	for n := range newNodes {
		if !oldNodes[n] {
			attr, _ := new.GetNodeAttr(n)
			result.Changes = append(result.Changes, Change{
				Kind:    ChangeAddNode,
				NodeID:  n,
				NewAttr: &attr,
			})
			result.Added++
		}
	}

	// Nodes removed
	for n := range oldNodes {
		if !newNodes[n] {
			attr, _ := old.GetNodeAttr(n)
			result.Changes = append(result.Changes, Change{
				Kind:    ChangeRemoveNode,
				NodeID:  n,
				OldAttr: &attr,
			})
			result.Removed++
		}
	}

	// Nodes modified (attributes changed)
	for n := range oldNodes {
		if !newNodes[n] {
			continue
		}
		oldAttr, _ := old.GetNodeAttr(n)
		newAttr, _ := new.GetNodeAttr(n)
		if oldAttr.Layer != newAttr.Layer || oldAttr.Owner != newAttr.Owner {
			result.Changes = append(result.Changes, Change{
				Kind:    ChangeModifyNode,
				NodeID:  n,
				OldAttr: &oldAttr,
				NewAttr: &newAttr,
			})
			result.Modified++
		}
	}

	// Edges
	oldEdges := edgeSet(old)
	newEdges := edgeSet(new)

	for key, attr := range newEdges {
		if _, ok := oldEdges[key]; !ok {
			a := attr
			result.Changes = append(result.Changes, Change{
				Kind:    ChangeAddEdge,
				From:    key[0],
				To:      key[1],
				NewEdge: &a,
			})
			result.Added++
		}
	}

	for key, attr := range oldEdges {
		if _, ok := newEdges[key]; !ok {
			a := attr
			result.Changes = append(result.Changes, Change{
				Kind:    ChangeRemoveEdge,
				From:    key[0],
				To:      key[1],
				OldEdge: &a,
			})
			result.Removed++
		}
	}

	for key, oldAttr := range oldEdges {
		newAttr, ok := newEdges[key]
		if !ok {
			continue
		}
		if oldAttr.Transform != newAttr.Transform || oldAttr.Comment != newAttr.Comment {
			oa := oldAttr
			na := newAttr
			result.Changes = append(result.Changes, Change{
				Kind:    ChangeModifyEdge,
				From:    key[0],
				To:      key[1],
				OldEdge: &oa,
				NewEdge: &na,
			})
			result.Modified++
		}
	}

	return result
}

// IsEmpty returns true if no changes were detected.
func (d *DiffResult) IsEmpty() bool {
	return len(d.Changes) == 0
}

// Summary returns a human-readable summary.
func (d *DiffResult) Summary() string {
	if d.IsEmpty() {
		return "no changes"
	}
	return fmt.Sprintf("%d change(s): %d added, %d removed, %d modified",
		len(d.Changes), d.Added, d.Removed, d.Modified)
}

// AffectedNodes returns all node IDs that appear in any change.
func (d *DiffResult) AffectedNodes() []string {
	seen := map[string]bool{}
	for _, c := range d.Changes {
		if c.NodeID != "" {
			seen[c.NodeID] = true
		}
		if c.From != "" {
			seen[c.From] = true
		}
		if c.To != "" {
			seen[c.To] = true
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// FilterByKind returns changes of a specific kind.
func (d *DiffResult) FilterByKind(kind ChangeKind) []Change {
	var out []Change
	for _, c := range d.Changes {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	return out
}

func setOf(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

func edgeSet(g *graph.Graph) map[[2]string]graph.EdgeAttr {
	m := map[[2]string]graph.EdgeAttr{}
	for _, e := range g.Edges() {
		m[[2]string{e.From, e.To}] = e.Attr
	}
	return m
}
