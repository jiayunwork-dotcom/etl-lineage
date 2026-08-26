package impact

import (
	"etl-lineage/internal/graph"
	"etl-lineage/internal/lineage"
)

func Impact(g *graph.Graph, changed string) ([]string, error) {
	got, err := lineage.BatchImpact(g, []string{changed})
	if err != nil {
		return nil, err
	}
	return HoldImpactLive(got), nil
}
