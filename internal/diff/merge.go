package diff

import (
	"fmt"

	"etl-lineage/internal/graph"
)

// Conflict describes a merge conflict between two graph versions.
type Conflict struct {
	NodeID  string `json:"node_id,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Reason  string `json:"reason"`
}

// MergeResult holds the outcome of merging two graphs.
type MergeResult struct {
	Graph     *graph.Graph `json:"-"`
	Conflicts []Conflict   `json:"conflicts"`
}

// HasConflicts returns true if the merge produced conflicts.
func (mr *MergeResult) HasConflicts() bool {
	return len(mr.Conflicts) > 0
}

// Merge combines two graph versions (both derived from a common ancestor)
// into a single graph. Conflicts arise when both versions modify the same
// node/edge in incompatible ways.
//
// Strategy:
//   - Nodes in both: if attributes differ, take 'theirs' (second graph) and record conflict
//   - Nodes only in one: add to result
//   - Edges in both: if transform differs, take 'theirs' and record conflict
//   - Edges only in one: add to result if both endpoints exist
//   - DAG invariant is checked post-merge; cycle creation is a conflict
func Merge(ours, theirs *graph.Graph) *MergeResult {
	result := &MergeResult{Graph: graph.New()}

	ourNodes := setOf(ours.Nodes())
	theirNodes := setOf(theirs.Nodes())

	// Add all nodes from both graphs
	allNodes := map[string]bool{}
	for n := range ourNodes {
		allNodes[n] = true
	}
	for n := range theirNodes {
		allNodes[n] = true
	}

	var conflicts []Conflict

	for n := range allNodes {
		inOurs := ourNodes[n]
		inTheirs := theirNodes[n]

		switch {
		case inOurs && inTheirs:
			ourAttr, _ := ours.GetNodeAttr(n)
			theirAttr, _ := theirs.GetNodeAttr(n)
			// take theirs, but record conflict if different
			result.Graph.AddNodeWithAttr(n, theirAttr)
			if ourAttr.Layer != theirAttr.Layer || ourAttr.Owner != theirAttr.Owner {
				conflicts = append(conflicts, Conflict{
					NodeID: n,
					Reason: fmt.Sprintf("attribute conflict: ours={layer:%s,owner:%s} theirs={layer:%s,owner:%s}",
						ourAttr.Layer, ourAttr.Owner, theirAttr.Layer, theirAttr.Owner),
				})
			}
		case inOurs:
			attr, _ := ours.GetNodeAttr(n)
			result.Graph.AddNodeWithAttr(n, attr)
		case inTheirs:
			attr, _ := theirs.GetNodeAttr(n)
			result.Graph.AddNodeWithAttr(n, attr)
		}
	}

	// Merge edges
	ourEdges := edgeSet(ours)
	theirEdges := edgeSet(theirs)
	allEdges := map[[2]string]bool{}
	for k := range ourEdges {
		allEdges[k] = true
	}
	for k := range theirEdges {
		allEdges[k] = true
	}

	for key := range allEdges {
		from, to := key[0], key[1]
		if !result.Graph.HasNode(from) || !result.Graph.HasNode(to) {
			continue
		}

		ourAttr, inOur := ourEdges[key]
		theirAttr, inTheir := theirEdges[key]

		switch {
		case inOur && inTheir:
			// both have the edge: take theirs, record conflict if different
			if err := result.Graph.AddEdgeWithAttr(from, to, theirAttr); err != nil {
				conflicts = append(conflicts, Conflict{
					From:   from,
					To:     to,
					Reason: fmt.Sprintf("edge would create cycle: %v", err),
				})
			} else if ourAttr.Transform != theirAttr.Transform {
				conflicts = append(conflicts, Conflict{
					From:   from,
					To:     to,
					Reason: fmt.Sprintf("transform conflict: ours=%s theirs=%s", ourAttr.Transform, theirAttr.Transform),
				})
			}
		case inOur:
			if err := result.Graph.AddEdgeWithAttr(from, to, ourAttr); err != nil {
				// edge from ours would create cycle in merged graph
				conflicts = append(conflicts, Conflict{
					From:   from,
					To:     to,
					Reason: fmt.Sprintf("ours-edge rejected: %v", err),
				})
			}
		case inTheir:
			if err := result.Graph.AddEdgeWithAttr(from, to, theirAttr); err != nil {
				conflicts = append(conflicts, Conflict{
					From:   from,
					To:     to,
					Reason: fmt.Sprintf("theirs-edge rejected: %v", err),
				})
			}
		}
	}

	result.Conflicts = conflicts
	return result
}

// ThreeWayMerge performs a three-way merge using a common ancestor (base).
// Changes from base→ours and base→theirs are reconciled into a single graph.
func ThreeWayMerge(base, ours, theirs *graph.Graph) *MergeResult {
	ourDiff := Compare(base, ours)
	theirDiff := Compare(base, theirs)

	// If one side has no changes, just return the other
	if ourDiff.IsEmpty() {
		return &MergeResult{Graph: theirs.Clone(), Conflicts: nil}
	}
	if theirDiff.IsEmpty() {
		return &MergeResult{Graph: ours.Clone(), Conflicts: nil}
	}

	// Both sides changed: fall back to direct merge
	return Merge(ours, theirs)
}
