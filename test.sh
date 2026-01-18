#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

show_help() {
    echo -e "${BLUE}Go Test & Benchmark Runner${NC}"
    echo ""
    echo -e "${GREEN}USAGE:${NC}"
    echo "    ./test.sh [DIRECTORIES] [OPTIONS]"
    echo ""
    echo -e "${GREEN}DIRECTORIES:${NC}"
    echo "    Can specify one or more directories to test:"
    echo "      ./test.sh lib/cron                    # Single directory"
    echo "      ./test.sh lib/cron lib/scheduler/index # Multiple directories"
    echo "      ./test.sh lib/*/                      # Glob pattern"
    echo "      ./test.sh .                           # Current directory (default)"
    echo ""
    echo -e "${GREEN}TEST OPTIONS:${NC}"
    echo "    --test [PATTERN]        Run tests, optionally filtered by pattern"
    echo "                            Examples:"
    echo "                              ./test.sh --test              # Run all tests"
    echo "                              ./test.sh --test Parse        # Run tests matching \"Parse\""
    echo "                              ./test.sh lib/cron --test    # Test specific directory"
    echo ""
    echo -e "${GREEN}BENCHMARK OPTIONS:${NC}"
    echo "    --bench [PATTERN]       Run benchmarks, optionally filtered by pattern"
    echo "                            Examples:"
    echo "                              ./test.sh --bench             # Run all benchmarks"
    echo "                              ./test.sh --bench Parse       # Run benchmarks matching \"Parse\""
    echo "                              ./test.sh lib/* --bench       # Benchmark all lib subdirs"
    echo ""
    echo -e "${GREEN}COMMON OPTIONS:${NC}"
    echo "    --mem                   Show memory allocation statistics (for benchmarks)"
    echo "    --no-cache              Disable test result caching (adds -count=1)"
    echo "    --timeout DURATION      Set timeout (default: 30m for benchmarks, 10m for tests)"
    echo "    -v, --verbose           Verbose test output"
    echo "    --help                  Show this help message"
    echo ""
    echo -e "${GREEN}EXAMPLES:${NC}"
    echo "    # Run all tests in current directory"
    echo "    ./test.sh --test"
    echo ""
    echo "    # Run tests in specific directory"
    echo "    ./test.sh lib/cron --test"
    echo ""
    echo "    # Run tests in multiple directories"
    echo "    ./test.sh lib/cron lib/scheduler/index --test"
    echo ""
    echo "    # Run tests matching \"Parse\" in all lib subdirectories"
    echo "    ./test.sh lib/*/ --test Parse"
    echo ""
    echo "    # Run all benchmarks in specific directory with memory stats"
    echo "    ./test.sh lib/cron --bench --mem"
    echo ""
    echo "    # Run benchmarks matching pattern across multiple dirs"
    echo "    ./test.sh lib/cron lib/scheduler/index --bench Parse --mem"
    echo ""
}

error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

success() {
    echo -e "${GREEN}$1${NC}"
}

info() {
    echo -e "${BLUE}$1${NC}"
}

warn() {
    echo -e "${YELLOW}$1${NC}"
}

# Build go test command with common flags
build_test_cmd() {
    local cmd="go test"

    if [[ "$VERBOSE" == "true" ]]; then
        cmd="$cmd -v"
    fi

    if [[ "$NO_CACHE" == "true" ]]; then
        cmd="$cmd -count=1"
    fi

    if [[ -n "$TIMEOUT" ]]; then
        cmd="$cmd -timeout $TIMEOUT"
    fi

    echo "$cmd"
}

# Run tests in a directory
run_tests() {
    local dir=$1
    local pattern=$2
    local cmd=$(build_test_cmd)

    if [[ -z "$TIMEOUT" ]]; then
        cmd="$cmd -timeout 10m"
    fi

    if [[ -n "$pattern" ]]; then
        info "Running tests matching: $pattern in $dir"
        cmd="$cmd -run $pattern"
    else
        info "Running all tests in $dir"
    fi

    cmd="$cmd ."

    # Always cd from script directory
    cd "$SCRIPT_DIR"
    cd "$dir"
    if eval $cmd; then
        success "✓ Tests passed in $dir"
        return 0
    else
        error "✗ Tests failed in $dir"
        return 1
    fi
}

# Run benchmarks in a directory
run_bench() {
    local dir=$1
    local pattern=$2
    local cmd=$(build_test_cmd)

    if [[ -z "$TIMEOUT" ]]; then
        cmd="$cmd -timeout 30m"
    fi

    if [[ -n "$pattern" ]]; then
        info "Running benchmarks matching: $pattern in $dir"
        cmd="$cmd -bench=$pattern"
    else
        info "Running all benchmarks in $dir"
        cmd="$cmd -bench=."
    fi

    if [[ "$SHOW_MEM" == "true" ]]; then
        cmd="$cmd -benchmem"
    fi

    cmd="$cmd ."

    # Always cd from script directory
    cd "$SCRIPT_DIR"
    cd "$dir"
    if eval $cmd; then
        success "✓ Benchmarks completed in $dir"
        return 0
    else
        error "✗ Benchmarks failed in $dir"
        return 1
    fi
}

# Parse arguments
SHOW_MEM=false
VERBOSE=false
NO_CACHE=false
TIMEOUT=""
ACTION=""
ACTION_ARG=""
DIRECTORIES=()

# First pass: collect directories and options
while [[ $# -gt 0 ]]; do
    case $1 in
        --help|-h)
            show_help
            exit 0
            ;;
        --test)
            ACTION="test"
            shift
            # Check if next arg is a pattern (not a flag)
            if [[ $# -gt 0 && ! $1 =~ ^-- ]]; then
                ACTION_ARG="$1"
                shift
            fi
            ;;
        --bench)
            ACTION="bench"
            shift
            # Check if next arg is a pattern (not a flag)
            if [[ $# -gt 0 && ! $1 =~ ^-- ]]; then
                ACTION_ARG="$1"
                shift
            fi
            ;;
        --mem)
            SHOW_MEM=true
            shift
            ;;
        --no-cache)
            NO_CACHE=true
            shift
            ;;
        --timeout)
            shift
            if [[ $# -eq 0 ]]; then
                error "--timeout requires a duration argument"
            fi
            TIMEOUT=$1
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -*)
            error "Unknown option: $1. Use --help for usage information."
            ;;
        *)
            # It's a directory argument
            DIRECTORIES+=("$1")
            shift
            ;;
    esac
done

# If no directories specified, use current directory
if [[ ${#DIRECTORIES[@]} -eq 0 ]]; then
    DIRECTORIES=(".")
fi

# Validate directories exist and find all subdirectories with Go test files
VALID_DIRS=()
for dir in "${DIRECTORIES[@]}"; do
    if [[ ! -d "$dir" ]]; then
        warn "Skipping $dir (not a directory)"
        continue
    fi

    # Recursively find all directories containing Go test files
    while IFS= read -r -d '' test_dir; do
        VALID_DIRS+=("$test_dir")
    done < <(find "$dir" -type f -name '*_test.go' -print0 | xargs -0 -n1 dirname | sort -u | tr '\n' '\0')
done

if [[ ${#VALID_DIRS[@]} -eq 0 ]]; then
    error "No valid directories with Go test files found"
fi

# Show what we're going to do
if [[ ${#VALID_DIRS[@]} -gt 1 ]]; then
    info "Will run in ${#VALID_DIRS[@]} directories: ${VALID_DIRS[*]}"
fi

# Execute the action for each directory
case $ACTION in
    test)
        for dir in "${VALID_DIRS[@]}"; do
            run_tests "$dir" "$ACTION_ARG"
        done
        success "✓ All tests completed successfully"
        ;;
    bench)
        for dir in "${VALID_DIRS[@]}"; do
            run_bench "$dir" "$ACTION_ARG"
        done
        success "✓ All benchmarks completed successfully"
        ;;
    "")
        if [[ ${#VALID_DIRS[@]} -eq 1 ]]; then
            info "No action specified. Use --test or --bench"
        else
            info "No action specified. Use --test or --bench with directories:"
            for dir in "${VALID_DIRS[@]}"; do
                echo "  - $dir"
            done
        fi
        echo ""
        echo "Use --help for more information."
        ;;
    *)
        error "Unknown action: $ACTION"
        ;;
esac
