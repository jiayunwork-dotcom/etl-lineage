#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_NAME="etl-lineage:benzhi"

docker build -f "${SCRIPT_DIR}/benzhi.Dockerfile" -t "${IMAGE_NAME}" "${SCRIPT_DIR}"
echo "Built image: ${IMAGE_NAME}"
echo "Run tests:   docker run --rm ${IMAGE_NAME}"
echo "Run binary:  docker run --rm ${IMAGE_NAME} etl-lineage --help"
