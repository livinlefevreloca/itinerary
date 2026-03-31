#!/bin/bash

set -e

echo "Running tests for Itinerary project..."
echo ""

# Run tests with race detection and coverage
echo "=== Running tests with race detection and coverage ==="
go test -race -coverprofile=coverage.out ./...

echo ""
echo "=== Coverage Summary ==="
go tool cover -func=coverage.out | grep total

echo ""
echo "=== Tests completed successfully ==="
