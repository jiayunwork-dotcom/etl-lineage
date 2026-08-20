# etl-lineage - Benzhi Evaluation

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
