package policy

import (
	"fmt"

	"etl-lineage/internal/graph"
)

type LayerFlowRule struct{}

func (r *LayerFlowRule) Name() string { return "layer_flow" }

func (r *LayerFlowRule) Evaluate(g *graph.Graph) []Violation {
	layerRank := map[string]int{
		"staging": 0,
		"dim":     1,
		"fact":    2,
		"report":  3,
	}
	var viols []Violation
	for _, e := range g.Edges() {
		fromAttr, err1 := g.GetNodeAttr(e.From)
		toAttr, err2 := g.GetNodeAttr(e.To)
		if err1 != nil || err2 != nil {
			continue
		}
		if fromAttr.Layer == "" || toAttr.Layer == "" {
			continue
		}
		fromRank, ok1 := layerRank[fromAttr.Layer]
		toRank, ok2 := layerRank[toAttr.Layer]
		if !ok1 || !ok2 {
			continue
		}
		if fromRank > toRank {
			viols = append(viols, Violation{
				Rule:     r.Name(),
				Severity: SeverityError,
				Message: fmt.Sprintf("edge %s->%s flows from %s(rank %d) to %s(rank %d)",
					e.From, e.To, fromAttr.Layer, fromRank, toAttr.Layer, toRank),
				Edge: [2]string{e.From, e.To},
			})
		}
	}
	return viols
}

type OwnerBoundaryRule struct {
	MaxCrossTeam int
}

func (r *OwnerBoundaryRule) Name() string { return "owner_boundary" }

func (r *OwnerBoundaryRule) Evaluate(g *graph.Graph) []Violation {
	crossTeam := 0
	var viols []Violation
	for _, e := range g.Edges() {
		fromAttr, err1 := g.GetNodeAttr(e.From)
		toAttr, err2 := g.GetNodeAttr(e.To)
		if err1 != nil || err2 != nil {
			continue
		}
		if fromAttr.Owner == "" || toAttr.Owner == "" {
			continue
		}
		if fromAttr.Owner != toAttr.Owner {
			crossTeam++
		}
	}
	if r.MaxCrossTeam > 0 && crossTeam > r.MaxCrossTeam {
		viols = append(viols, Violation{
			Rule:     r.Name(),
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("graph has %d cross-team edges (max allowed: %d)", crossTeam, r.MaxCrossTeam),
		})
	}
	return viols
}

type TransformAllowlistRule struct {
	Allowed []graph.TransformType
}

func (r *TransformAllowlistRule) Name() string { return "transform_allowlist" }

func (r *TransformAllowlistRule) Evaluate(g *graph.Graph) []Violation {
	allowed := map[graph.TransformType]bool{}
	for _, t := range r.Allowed {
		allowed[t] = true
	}
	var viols []Violation
	for _, e := range g.Edges() {
		if !allowed[e.Attr.Transform] {
			viols = append(viols, Violation{
				Rule:     r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("edge %s->%s uses disallowed transform %q", e.From, e.To, e.Attr.Transform),
				Edge:     [2]string{e.From, e.To},
			})
		}
	}
	return viols
}

type MaxFanInRule struct {
	MaxFanIn int
}

func (r *MaxFanInRule) Name() string { return "max_fan_in" }

func (r *MaxFanInRule) Evaluate(g *graph.Graph) []Violation {
	var viols []Violation
	for _, n := range g.Nodes() {
		preds := g.Predecessors(n)
		if len(preds) > r.MaxFanIn {
			viols = append(viols, Violation{
				Rule:     r.Name(),
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has fan-in %d (max %d)", n, len(preds), r.MaxFanIn),
				Nodes:    []string{n},
			})
		}
	}
	return viols
}

type MaxFanOutRule struct {
	MaxFanOut int
}

func (r *MaxFanOutRule) Name() string { return "max_fan_out" }

func (r *MaxFanOutRule) Evaluate(g *graph.Graph) []Violation {
	var viols []Violation
	for _, n := range g.Nodes() {
		succs := g.Successors(n)
		if len(succs) > r.MaxFanOut {
			viols = append(viols, Violation{
				Rule:     r.Name(),
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has fan-out %d (max %d)", n, len(succs), r.MaxFanOut),
				Nodes:    []string{n},
			})
		}
	}
	return viols
}

type NoSelfOwnerRule struct{}

func (r *NoSelfOwnerRule) Name() string { return "no_self_dependency" }

func (r *NoSelfOwnerRule) Evaluate(g *graph.Graph) []Violation {
	var viols []Violation
	for _, e := range g.Edges() {
		if e.From == e.To {
			viols = append(viols, Violation{
				Rule:     r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("self-loop detected: %s->%s", e.From, e.To),
				Nodes:    []string{e.From},
			})
		}
	}
	return viols
}
