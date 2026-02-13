#!/bin/bash

set -e

echo "Running tests for Itinerary project..."
echo ""

# Run tests with race detection and coverage
# Filter out harmless macOS linker warnings from sqlite3
echo "=== Running tests with race detection and coverage ==="
go test -race -coverprofile=coverage.out ./... 2>&1 | grep -v "ld: warning.*LC_DYSYMTAB"

echo ""
echo "=== Coverage Summary ==="
go tool cover -func=coverage.out | grep total

echo ""
echo "=== Tests completed successfully ==="
