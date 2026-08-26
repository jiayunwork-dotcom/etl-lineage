# etl-lineage — Go ETL 数据血缘图谱与影响分析 HTTP 服务（含 WAL 持久化）

读入表级/字段级血缘声明，建成 DAG 后做上下游追溯、变更影响与拓扑调度；规格非法、成环或 WAL 未落稳时返回错误，不得只报端口。同一节点变更须同时算出受影响下游与可执行顺序。

## Build

```bash
./build_benzhi_docker.sh
```

## Run Tests

```bash
docker run --rm etl-lineage:benzhi
```

## Run Binary

```bash
docker run --rm etl-lineage:benzhi etl-lineage -spec /app/example/spec.txt
docker run --rm etl-lineage:benzhi etl-lineage -spec /app/example/spec.txt -node dim_orders
docker run --rm etl-lineage:benzhi etl-lineage validate -spec /app/example/spec.txt
docker run --rm etl-lineage:benzhi etl-lineage report -spec /app/example/spec.txt -format json
docker run --rm etl-lineage:benzhi etl-lineage stats -spec /app/example/spec.txt
```

## Environment

- Base image: `golang:1.21-alpine`
- `GOTOOLCHAIN=local`
- `CGO_ENABLED=0`
