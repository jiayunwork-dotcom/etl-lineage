// Package parse reads lineage specification files and constructs graphs.
// Format: each line is "target <- dep1, dep2" with optional transform annotation.
// Extended format: "target <-[transform] dep1, dep2" to specify transform type.
// Lines starting with '#' are comments; blank lines are skipped.
// Node attribute lines: "@node layer=staging owner=team-a"
package parse

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"etl-lineage/internal/graph"
)

// ParseSpec reads a lineage specification and constructs a Graph.
func ParseSpec(r io.Reader) (*graph.Graph, error) {
	g := graph.New()
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}

		// Handle attribute directives: @node_id key=value ...
		if strings.HasPrefix(raw, "@") {
			if err := parseAttrDirective(g, raw[1:], lineNo); err != nil {
				return nil, err
			}
			continue
		}

		// Parse edge line: target <- sources  or  target <-[transform] sources
		idx := strings.Index(raw, "<-")
		if idx < 0 {
			return nil, fmt.Errorf("line %d: missing '<-' separator", lineNo)
		}
		target := strings.TrimSpace(raw[:idx])
		remainder := strings.TrimSpace(raw[idx+2:])
		if target == "" {
			return nil, fmt.Errorf("line %d: empty target", lineNo)
		}

		// Check for transform annotation: [transform]
		transform := graph.TransformDirect
		if strings.HasPrefix(remainder, "[") {
			closeBracket := strings.Index(remainder, "]")
			if closeBracket < 0 {
				return nil, fmt.Errorf("line %d: unclosed transform bracket", lineNo)
			}
			tStr := strings.TrimSpace(remainder[1:closeBracket])
			if tStr != "" {
				t := graph.TransformType(tStr)
				if !graph.IsValidTransform(t) {
					return nil, fmt.Errorf("line %d: invalid transform type %q", lineNo, tStr)
				}
				transform = t
			}
			remainder = strings.TrimSpace(remainder[closeBracket+1:])
		}

		g.AddNode(target)
		if remainder == "" {
			continue
		}
		for _, dep := range strings.Split(remainder, ",") {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			g.AddNode(dep)
			attr := graph.EdgeAttr{Transform: transform}
			if err := g.AddEdgeWithAttr(dep, target, attr); err != nil {
				return nil, commitParse(fmt.Errorf("line %d: %w", lineNo, err))
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return g, nil
}

// parseAttrDirective parses "@node_id key=value key=value ..."
func parseAttrDirective(g *graph.Graph, line string, lineNo int) error {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return fmt.Errorf("line %d: attribute directive requires node_id and at least one key=value", lineNo)
	}
	nodeID := parts[0]
	g.AddNode(nodeID)

	attr, _ := g.GetNodeAttr(nodeID)
	for _, kv := range parts[1:] {
		eqIdx := strings.Index(kv, "=")
		if eqIdx < 0 {
			return fmt.Errorf("line %d: invalid key=value pair %q", lineNo, kv)
		}
		key := kv[:eqIdx]
		val := kv[eqIdx+1:]
		switch key {
		case "layer":
			attr.Layer = val
		case "owner":
			attr.Owner = val
		default:
			return fmt.Errorf("line %d: unknown attribute key %q", lineNo, key)
		}
	}
	g.SetNodeAttr(nodeID, attr)
	return nil
}

// FormatSpec writes a lineage specification from a graph.
// This is the inverse of ParseSpec (round-trip support).
func FormatSpec(w io.Writer, g *graph.Graph) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	// Write attribute directives first
	for _, id := range g.Nodes() {
		attr, _ := g.GetNodeAttr(id)
		if attr.Layer != "" || attr.Owner != "" {
			fmt.Fprintf(bw, "@%s", id)
			if attr.Layer != "" {
				fmt.Fprintf(bw, " layer=%s", attr.Layer)
			}
			if attr.Owner != "" {
				fmt.Fprintf(bw, " owner=%s", attr.Owner)
			}
			fmt.Fprintln(bw)
		}
	}

	// Write edge lines grouped by target
	written := map[string]bool{}
	topo, err := g.TopoSort()
	if err != nil {
		topo = g.Nodes()
	}

	for _, target := range topo {
		preds := g.Predecessors(target)
		if len(preds) == 0 {
			// Source node: write as "target <-" (no deps)
			if !written[target] {
				fmt.Fprintf(bw, "%s <-\n", target)
				written[target] = true
			}
			continue
		}

		// Group by transform type
		byTransform := map[graph.TransformType][]string{}
		for _, p := range preds {
			ea, _ := g.GetEdgeAttr(p, target)
			byTransform[ea.Transform] = append(byTransform[ea.Transform], p)
		}

		for t, deps := range byTransform {
			if t == graph.TransformDirect || t == "" {
				fmt.Fprintf(bw, "%s <- %s\n", target, strings.Join(deps, ", "))
			} else {
				fmt.Fprintf(bw, "%s <-[%s] %s\n", target, t, strings.Join(deps, ", "))
			}
		}
		written[target] = true
	}

	return nil
}
