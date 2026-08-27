package impact

import (
	"etl-lineage/internal/graph"
	"etl-lineage/internal/lineage"
)

func Impact(g *graph.Graph, changed string) ([]string, error) {
	return lineage.BatchImpact(g, []string{changed})
}
