package diff

import (
	"fmt"

	"etl-lineage/internal/graph"
)

type Conflict struct {
	NodeID string `json:"node_id,omitempty"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Reason string `json:"reason"`
}

type MergeResult struct {
	Graph     *graph.Graph `json:"-"`
	Conflicts []Conflict   `json:"conflicts"`
}

func (mr *MergeResult) HasConflicts() bool {
	return len(mr.Conflicts) > 0
}

func Merge(ours, theirs *graph.Graph) *MergeResult {
	result := &MergeResult{Graph: graph.New()}

	ourNodes := setOf(ours.Nodes())
	theirNodes := setOf(theirs.Nodes())

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

func ThreeWayMerge(base, ours, theirs *graph.Graph) *MergeResult {
	ourDiff := Compare(base, ours)
	theirDiff := Compare(base, theirs)

	if ourDiff.IsEmpty() {
		return &MergeResult{Graph: theirs.Clone(), Conflicts: nil}
	}
	if theirDiff.IsEmpty() {
		return &MergeResult{Graph: ours.Clone(), Conflicts: nil}
	}

	return Merge(ours, theirs)
}
