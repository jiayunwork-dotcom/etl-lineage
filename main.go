package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"etl-lineage/internal/graph"
	"etl-lineage/internal/lineage"
	"etl-lineage/internal/parse"
	"etl-lineage/internal/report"
	"etl-lineage/internal/store"
	"etl-lineage/internal/validate"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "etl-lineage:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return runDefault(args, stdout)
	}
	switch args[0] {
	case "impact":
		return runImpact(args[1:], stdout)
	case "validate":
		return runValidate(args[1:], stdout)
	case "report":
		return runReport(args[1:], stdout)
	case "store":
		return runStore(args[1:], stdout)
	case "stats":
		return runStats(args[1:], stdout)
	default:
		return runDefault(args, stdout)
	}
}

func runDefault(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("etl-lineage", flag.ContinueOnError)
	spec := fs.String("spec", "-", "lineage spec file ('-' for stdin)")
	query := fs.String("node", "", "node to compute downstream impact for")
	dot := fs.String("dot", "-", "write DOT graph ('-' for stdout)")
	format := fs.String("format", "dot", "output format: dot, json, text")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := loadSpec(*spec)
	if err != nil {
		return err
	}

	if *query != "" {
		imp, err := lineage.BatchImpact(g, strings.Split(*query, ","))
		if err != nil {
			return fmt.Errorf("impact: %w", err)
		}
		for _, n := range imp {
			fmt.Fprintln(stdout, n)
		}
		return nil
	}

	switch *format {
	case "json":
		return report.WriteJSON(stdout, g)
	case "text":
		return report.WriteTextSummary(stdout, g)
	default:
		var w io.Writer = stdout
		if *dot != "-" {
			of, err := os.Create(*dot)
			if err != nil {
				return fmt.Errorf("create dot: %w", err)
			}
			defer of.Close()
			w = of
		}
		return report.WriteDOT(w, g)
	}
}

func runImpact(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("impact", flag.ContinueOnError)
	spec := fs.String("spec", "-", "lineage spec file")
	nodes := fs.String("nodes", "", "comma-separated changed nodes")
	format := fs.String("format", "text", "output format: text, json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *nodes == "" {
		return fmt.Errorf("impact: -nodes is required")
	}

	g, err := loadSpec(*spec)
	if err != nil {
		return err
	}

	changed := strings.Split(*nodes, ",")
	for i := range changed {
		changed[i] = strings.TrimSpace(changed[i])
	}

	switch *format {
	case "json":
		return report.WriteImpactSummary(stdout, g, changed)
	default:
		imp, err := lineage.BatchImpact(g, changed)
		if err != nil {
			return fmt.Errorf("impact: %w", err)
		}
		if len(imp) == 0 {
			fmt.Fprintln(stdout, "(no downstream impact)")
			return nil
		}
		for _, n := range imp {
			fmt.Fprintln(stdout, n)
		}
		return nil
	}
}

func runValidate(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	spec := fs.String("spec", "-", "lineage spec file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := loadSpec(*spec)
	if err != nil {
		return err
	}

	result := validate.ValidateComplete(g)
	if result.OK() {
		fmt.Fprintln(stdout, "PASS")
	} else {
		fmt.Fprintln(stdout, "FAIL")
	}
	for _, iss := range result.Issues {
		fmt.Fprintf(stdout, "  [%s] %s: %s\n", strings.ToUpper(iss.Severity), iss.Code, iss.Message)
	}
	if !result.OK() {
		return fmt.Errorf("validation failed with %d error(s)", len(result.Errors()))
	}
	return nil
}

func runReport(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	spec := fs.String("spec", "-", "lineage spec file")
	format := fs.String("format", "json", "output format: json, text, dot, matrix")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := loadSpec(*spec)
	if err != nil {
		return err
	}

	switch *format {
	case "text":
		return report.WriteTextSummary(stdout, g)
	case "dot":
		return report.WriteDOT(stdout, g)
	case "matrix":
		return report.WriteDepMatrix(stdout, g)
	default:
		return report.WriteJSON(stdout, g)
	}
}

func runStore(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("store: subcommand required (init, load, compact, export, show)")
	}
	sub := args[0]
	fs := flag.NewFlagSet("store "+sub, flag.ContinueOnError)
	dir := fs.String("dir", ".lineage-store", "store directory path")
	spec := fs.String("spec", "", "lineage spec file to load")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	switch sub {
	case "init":
		s, err := store.Open(*dir)
		if err != nil {
			return fmt.Errorf("store init: %w", err)
		}
		s.Close()
		fmt.Fprintf(stdout, "store initialized at %s\n", *dir)
		return nil

	case "load":
		if *spec == "" {
			return fmt.Errorf("store load: -spec is required")
		}
		g, err := loadSpec(*spec)
		if err != nil {
			return err
		}
		s, err := store.Open(*dir)
		if err != nil {
			return fmt.Errorf("store open: %w", err)
		}
		defer s.Close()
		// Load all nodes and edges
		for _, id := range g.Nodes() {
			attr, _ := g.GetNodeAttr(id)
			s.AddNode(id, &attr)
		}
		for _, e := range g.Edges() {
			ea := e.Attr
			s.AddEdge(e.From, e.To, &ea)
		}
		if err := s.Compact(); err != nil {
			return fmt.Errorf("store compact: %w", err)
		}
		fmt.Fprintf(stdout, "loaded %d nodes, %d edges into store\n", g.NodeCount(), g.EdgeCount())
		return nil

	case "compact":
		s, err := store.Open(*dir)
		if err != nil {
			return fmt.Errorf("store open: %w", err)
		}
		defer s.Close()
		walBefore := s.WALSize()
		if err := s.Compact(); err != nil {
			return fmt.Errorf("store compact: %w", err)
		}
		fmt.Fprintf(stdout, "compacted: WAL %d -> 0 bytes\n", walBefore)
		return nil

	case "export":
		s, err := store.Open(*dir)
		if err != nil {
			return fmt.Errorf("store open: %w", err)
		}
		defer s.Close()
		snap, err := s.Snapshot()
		if err != nil {
			return fmt.Errorf("store snapshot: %w", err)
		}
		stdout.Write(snap)
		fmt.Fprintln(stdout)
		return nil

	case "show":
		s, err := store.Open(*dir)
		if err != nil {
			return fmt.Errorf("store open: %w", err)
		}
		defer s.Close()
		g := s.Graph()
		return report.WriteTextSummary(stdout, g)

	default:
		return fmt.Errorf("store: unknown subcommand %q", sub)
	}
}

func runStats(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	spec := fs.String("spec", "-", "lineage spec file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	g, err := loadSpec(*spec)
	if err != nil {
		return err
	}

	return report.WriteStats(stdout, g)
}

func loadSpec(path string) (*graph.Graph, error) {
	var r io.Reader = os.Stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open spec: %w", err)
		}
		defer f.Close()
		r = f
	}
	g, err := parse.ParseSpec(r)
	if err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	return g, nil
}
