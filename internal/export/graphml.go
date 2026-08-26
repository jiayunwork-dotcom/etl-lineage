package export

import (
	"encoding/xml"
	"fmt"
	"io"

	"etl-lineage/internal/graph"
)

type graphMLDoc struct {
	XMLName xml.Name     `xml:"graphml"`
	XMLNS   string       `xml:"xmlns,attr"`
	Keys    []graphMLKey `xml:"key"`
	Graph   graphMLGraph `xml:"graph"`
}

type graphMLKey struct {
	ID   string `xml:"id,attr"`
	For  string `xml:"for,attr"`
	Name string `xml:"attr.name,attr"`
	Type string `xml:"attr.type,attr"`
}

type graphMLGraph struct {
	ID          string        `xml:"id,attr"`
	EdgeDefault string        `xml:"edgedefault,attr"`
	Nodes       []graphMLNode `xml:"node"`
	Edges       []graphMLEdge `xml:"edge"`
}

type graphMLNode struct {
	ID   string        `xml:"id,attr"`
	Data []graphMLData `xml:"data"`
}

type graphMLEdge struct {
	Source string        `xml:"source,attr"`
	Target string        `xml:"target,attr"`
	Data   []graphMLData `xml:"data"`
}

type graphMLData struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

func WriteGraphML(w io.Writer, g *graph.Graph) error {
	doc := graphMLDoc{
		XMLNS: "http://graphml.graphdml.org/xmlns",
		Keys: []graphMLKey{
			{ID: "d0", For: "node", Name: "layer", Type: "string"},
			{ID: "d1", For: "node", Name: "owner", Type: "string"},
			{ID: "d2", For: "edge", Name: "transform", Type: "string"},
			{ID: "d3", For: "edge", Name: "comment", Type: "string"},
		},
		Graph: graphMLGraph{
			ID:          "lineage",
			EdgeDefault: "directed",
		},
	}

	for _, n := range g.Nodes() {
		attr, _ := g.GetNodeAttr(n)
		node := graphMLNode{ID: n}
		if attr.Layer != "" {
			node.Data = append(node.Data, graphMLData{Key: "d0", Value: attr.Layer})
		}
		if attr.Owner != "" {
			node.Data = append(node.Data, graphMLData{Key: "d1", Value: attr.Owner})
		}
		doc.Graph.Nodes = append(doc.Graph.Nodes, node)
	}

	for _, e := range g.Edges() {
		edge := graphMLEdge{Source: e.From, Target: e.To}
		edge.Data = append(edge.Data, graphMLData{Key: "d2", Value: string(e.Attr.Transform)})
		if e.Attr.Comment != "" {
			edge.Data = append(edge.Data, graphMLData{Key: "d3", Value: e.Attr.Comment})
		}
		doc.Graph.Edges = append(doc.Graph.Edges, edge)
	}

	fmt.Fprint(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("export graphml: %w", err)
	}
	return nil
}

func WriteGEXF(w io.Writer, g *graph.Graph) error {
	fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(w, `<gexf xmlns="http://gexf.net/1.3" version="1.3">`)
	fmt.Fprintln(w, `  <graph mode="static" defaultedgetype="directed">`)
	fmt.Fprintln(w, `    <nodes>`)
	for _, n := range g.Nodes() {
		attr, _ := g.GetNodeAttr(n)
		label := n
		if attr.Layer != "" {
			label = n + " [" + attr.Layer + "]"
		}
		fmt.Fprintf(w, "      <node id=%q label=%q/>\n", n, label)
	}
	fmt.Fprintln(w, `    </nodes>`)
	fmt.Fprintln(w, `    <edges>`)
	for i, e := range g.Edges() {
		fmt.Fprintf(w, "      <edge id=%q source=%q target=%q label=%q/>\n",
			fmt.Sprintf("e%d", i), e.From, e.To, string(e.Attr.Transform))
	}
	fmt.Fprintln(w, `    </edges>`)
	fmt.Fprintln(w, `  </graph>`)
	fmt.Fprintln(w, `</gexf>`)
	return nil
}
