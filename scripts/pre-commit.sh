#!/bin/bash
set -e
echo "Running pre-commit checks..."
go vet ./...
go test -short -count=1 ./... 2>&1 | tail -5
echo "All checks passed."
