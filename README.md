# etl-lineage

ETL lineage tracking and impact analysis tool (CLI).

Parses declarative lineage specifications, builds a directed acyclic graph (DAG) of table-level and column-level dependencies, and provides topological scheduling, change impact analysis, schema drift detection, policy enforcement, and multi-format export.

## Architecture

```
parse (spec) → graph (DAG core)
                  ├── lineage (upstream/downstream/paths)
                  ├── impact (batch change analysis)
                  ├── scheduler (topo execution + cancellation)
                  ├── diff (version comparison + merge)
                  ├── column (field-level lineage)
                  ├── policy (governance rules engine)
                  ├── metrics (health scores + hotspots)
                  ├── validate (DAG invariant checks)
                  ├── export (CSV/YAML/SQL/GraphML/GEXF)
                  └── report (DOT/JSON/text/matrix)
              store (WAL + snapshot persistence)
```

Packages:

| Package | Role |
|---------|------|
| `internal/graph` | DAG core: nodes/edges with attributes, cycle detection, topo sort, subgraph, serialization |
| `internal/parse` | Declarative spec parser (`target <- dep1, dep2` format) |
| `internal/lineage` | Upstream/downstream traversal, batch impact, path enumeration, critical path |
| `internal/impact` | Single-node impact convenience wrapper |
| `internal/scheduler` | Topological execution scheduler with context cancellation and dependency-failure skip |
| `internal/diff` | Graph version comparison (add/remove/modify) and three-way merge with conflict detection |
| `internal/column` | Column-level (field) lineage: mapping tracking, transitive tracing, persistence |
| `internal/policy` | Governance rule engine: layer flow, owner boundary, transform allowlist, fan-in/out limits |
| `internal/metrics` | Structural metrics (density, depth, coupling, complexity score) and health checks |
| `internal/validate` | DAG integrity, orphan detection, layer consistency, naming conventions |
| `internal/export` | Multi-format export: CSV, YAML, SQL DDL, GraphML, GEXF, node list |
| `internal/store` | Append-only WAL + snapshot persistence with crash recovery and compaction |
| `internal/report` | DOT, JSON, text summary, dependency matrix output |

## Usage

```bash
# Default: parse spec and output DOT graph
etl-lineage -spec pipeline.txt -format dot

# Impact analysis: which tables are affected by changes to src_orders?
etl-lineage impact -spec pipeline.txt -nodes src_orders

# Validate DAG integrity
etl-lineage validate -spec pipeline.txt

# Report in multiple formats
etl-lineage report -spec pipeline.txt -format json
etl-lineage report -spec pipeline.txt -format matrix

# Persistent store operations
etl-lineage store init -dir .lineage-store
etl-lineage store load -dir .lineage-store -spec pipeline.txt
etl-lineage store compact -dir .lineage-store
```

## Spec Format

```
# Table-level lineage
dim_customer <- stg_orders, stg_customers
fact_sales <-[aggregate] stg_orders
report_daily <-[join] fact_sales, dim_customer

# Node attributes
@stg_orders layer=staging owner=data-eng
@dim_customer layer=dim owner=data-eng
@fact_sales layer=fact owner=analytics
```

## Build & Test

```bash
export GOTOOLCHAIN=local CGO_ENABLED=0
go build ./...
go test ./...
```

## License

MIT
