// Package impact provides backward-compatible impact analysis entry point.
package impact

import (
	"etl-lineage/internal/graph"
	"etl-lineage/internal/lineage"
)

// Impact returns sorted downstream nodes affected by a change to the given node.
func Impact(g *graph.Graph, changed string) ([]string, error) {
	got, err := lineage.BatchImpact(g, []string{changed})
	if err != nil {
		return nil, err
	}
	return applyImpact(got), nil
}
