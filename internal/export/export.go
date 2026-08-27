package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"

	"etl-lineage/internal/graph"
)

func WriteCSV(w io.Writer, g *graph.Graph) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"from", "to", "transform", "comment"}); err != nil {
		return fmt.Errorf("export csv: header: %w", err)
	}
	for _, e := range g.Edges() {
		row := []string{e.From, e.To, string(e.Attr.Transform), e.Attr.Comment}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("export csv: row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

func WriteYAML(w io.Writer, g *graph.Graph) error {
	fmt.Fprintln(w, "nodes:")
	nodes := g.Nodes()
	sort.Strings(nodes)
	for _, n := range nodes {
		attr, _ := g.GetNodeAttr(n)
		fmt.Fprintf(w, "  - id: %s\n", n)
		if attr.Layer != "" {
			fmt.Fprintf(w, "    layer: %s\n", attr.Layer)
		}
		if attr.Owner != "" {
			fmt.Fprintf(w, "    owner: %s\n", attr.Owner)
		}
	}
	fmt.Fprintln(w, "edges:")
	for _, e := range g.Edges() {
		fmt.Fprintf(w, "  - from: %s\n    to: %s\n    transform: %s\n",
			e.From, e.To, e.Attr.Transform)
		if e.Attr.Comment != "" {
			fmt.Fprintf(w, "    comment: %s\n", e.Attr.Comment)
		}
	}
	return nil
}

func WriteSQLDDL(w io.Writer, g *graph.Graph) error {
	var sb strings.Builder
	sb.WriteString("CREATE TABLE IF NOT EXISTS lineage_nodes (\n")
	sb.WriteString("  id    VARCHAR(255) PRIMARY KEY,\n")
	sb.WriteString("  layer VARCHAR(50),\n")
	sb.WriteString("  owner VARCHAR(100)\n")
	sb.WriteString(");\n\n")
	sb.WriteString("CREATE TABLE IF NOT EXISTS lineage_edges (\n")
	sb.WriteString("  from_node  VARCHAR(255) NOT NULL REFERENCES lineage_nodes(id),\n")
	sb.WriteString("  to_node    VARCHAR(255) NOT NULL REFERENCES lineage_nodes(id),\n")
	sb.WriteString("  transform  VARCHAR(50)  NOT NULL,\n")
	sb.WriteString("  comment    TEXT,\n")
	sb.WriteString("  PRIMARY KEY (from_node, to_node)\n")
	sb.WriteString(");\n\n")

	nodes := g.Nodes()
	sort.Strings(nodes)
	for _, n := range nodes {
		attr, _ := g.GetNodeAttr(n)
		sb.WriteString(fmt.Sprintf("INSERT INTO lineage_nodes (id, layer, owner) VALUES ('%s', '%s', '%s');\n",
			escapeSQLStr(n), escapeSQLStr(attr.Layer), escapeSQLStr(attr.Owner)))
	}
	sb.WriteString("\n")
	for _, e := range g.Edges() {
		sb.WriteString(fmt.Sprintf("INSERT INTO lineage_edges (from_node, to_node, transform, comment) VALUES ('%s', '%s', '%s', '%s');\n",
			escapeSQLStr(e.From), escapeSQLStr(e.To),
			escapeSQLStr(string(e.Attr.Transform)), escapeSQLStr(e.Attr.Comment)))
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

func WriteNodeList(w io.Writer, g *graph.Graph) error {
	order, err := g.TopoSort()
	if err != nil {
		return fmt.Errorf("export nodelist: %w", err)
	}
	for _, n := range order {
		fmt.Fprintln(w, n)
	}
	return nil
}

func escapeSQLStr(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
