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
    echo -e "${BLUE}Cron Parser Test & Benchmark Runner${NC}"
    echo ""
    echo -e "${GREEN}USAGE:${NC}"
    echo "    ./test.sh [OPTIONS]"
    echo ""
    echo -e "${GREEN}TEST OPTIONS:${NC}"
    echo "    --test [PATTERN]        Run tests, optionally filtered by pattern"
    echo "                            Examples:"
    echo "                              ./test.sh --test              # Run all tests"
    echo "                              ./test.sh --test Parse        # Run tests matching \"Parse\""
    echo "                              ./test.sh --test DayLogic     # Run day logic tests"
    echo ""
    echo -e "${GREEN}BENCHMARK OPTIONS:${NC}"
    echo "    --bench                 Enable benchmark mode (required for benchmark flags)"
    echo ""
    echo "    --parse N               Run parsing benchmarks for N schedules"
    echo "                            Valid values: 10, 100, 1000, 10000, 100000"
    echo "                            Example: ./test.sh --bench --parse 1000"
    echo ""
    echo "    --match N WINDOW        Run matching benchmarks for N schedules over WINDOW time"
    echo "                            Valid N: 10, 100, 1000, 10000"
    echo "                            Valid WINDOW: 1h, 1d, 1w, 1m, 1y"
    echo "                            Example: ./test.sh --bench --match 1000 1w"
    echo ""
    echo "    --pattern PATTERN       Run benchmarks matching PATTERN"
    echo "                            Example: ./test.sh --bench --pattern Next"
    echo ""
    echo "    --all-parse             Run all parsing benchmarks"
    echo "    --all-match             Run all matching benchmarks"
    echo "    --all                   Run all benchmarks"
    echo ""
    echo -e "${GREEN}COMMON OPTIONS:${NC}"
    echo "    --mem                   Show memory allocation statistics (for benchmarks)"
    echo "    --timeout DURATION      Set timeout (default: 30m for benchmarks, 10m for tests)"
    echo "    -v, --verbose           Verbose test output"
    echo "    --help                  Show this help message"
    echo ""
    echo -e "${GREEN}EXAMPLES:${NC}"
    echo "    # Run all tests"
    echo "    ./test.sh --test"
    echo ""
    echo "    # Run tests matching \"Parse\""
    echo "    ./test.sh --test Parse"
    echo ""
    echo "    # Run parsing benchmark for 1000 schedules"
    echo "    ./test.sh --bench --parse 1000"
    echo ""
    echo "    # Run matching benchmark for 10000 schedules over 1 week"
    echo "    ./test.sh --bench --match 10000 1w"
    echo ""
    echo "    # Run all parsing benchmarks with memory stats"
    echo "    ./test.sh --bench --all-parse --mem"
    echo ""
    echo "    # Run benchmarks matching \"Daily\""
    echo "    ./test.sh --bench --pattern Daily --mem"
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

# Validate parsing schedule count
validate_parse_count() {
    local count=$1
    case $count in
        10|100|1000|10000|100000)
            return 0
            ;;
        *)
            error "Invalid parsing schedule count: $count. Valid values: 10, 100, 1000, 10000, 100000"
            ;;
    esac
}

# Validate matching schedule count
validate_match_count() {
    local count=$1
    case $count in
        10|100|1000|10000)
            return 0
            ;;
        *)
            error "Invalid matching schedule count: $count. Valid values: 10, 100, 1000, 10000"
            ;;
    esac
}

# Validate time window
validate_window() {
    local window=$1
    case $window in
        1h|1d|1w|1m|1y)
            return 0
            ;;
        *)
            error "Invalid time window: $window. Valid values: 1h, 1d, 1w, 1m, 1y"
            ;;
    esac
}

# Convert window shorthand to benchmark suffix
window_to_suffix() {
    local window=$1
    case $window in
        1h) echo "1Hour" ;;
        1d) echo "1Day" ;;
        1w) echo "1Week" ;;
        1m) echo "1Month" ;;
        1y) echo "1Year" ;;
    esac
}

# Build go test command with common flags
build_test_cmd() {
    local cmd="go test"

    if [[ "$VERBOSE" == "true" ]]; then
        cmd="$cmd -v"
    fi

    if [[ -n "$TIMEOUT" ]]; then
        cmd="$cmd -timeout $TIMEOUT"
    fi

    echo "$cmd"
}

# Run tests
run_tests() {
    local pattern=$1
    local cmd=$(build_test_cmd)

    if [[ -z "$TIMEOUT" ]]; then
        cmd="$cmd -timeout 10m"
    fi

    if [[ -n "$pattern" ]]; then
        info "Running tests matching: $pattern"
        cmd="$cmd -run $pattern"
    else
        info "Running all tests"
    fi

    cmd="$cmd ."

    cd "$SCRIPT_DIR"
    eval $cmd
    success "✓ Tests passed"
}

# Run parsing benchmark
run_parse_bench() {
    local count=$1
    validate_parse_count "$count"

    local bench_name="BenchmarkParse_${count}_Schedules"
    info "Running parsing benchmark for $count schedules"

    local cmd=$(build_test_cmd)
    if [[ -z "$TIMEOUT" ]]; then
        cmd="$cmd -timeout 30m"
    fi

    cmd="$cmd -bench=^${bench_name}$"

    if [[ "$SHOW_MEM" == "true" ]]; then
        cmd="$cmd -benchmem"
    fi

    cmd="$cmd ."

    cd "$SCRIPT_DIR"
    eval $cmd
    success "✓ Benchmark completed"
}

# Run matching benchmark
run_match_bench() {
    local count=$1
    local window=$2

    validate_match_count "$count"
    validate_window "$window"

    local window_suffix=$(window_to_suffix "$window")
    local bench_name="BenchmarkMatch_${count}_Schedules_${window_suffix}"

    info "Running matching benchmark for $count schedules over $window"

    local cmd=$(build_test_cmd)
    if [[ -z "$TIMEOUT" ]]; then
        cmd="$cmd -timeout 30m"
    fi

    cmd="$cmd -bench=^${bench_name}$"

    if [[ "$SHOW_MEM" == "true" ]]; then
        cmd="$cmd -benchmem"
    fi

    cmd="$cmd ."

    cd "$SCRIPT_DIR"
    eval $cmd
    success "✓ Benchmark completed"
}

# Run benchmark by pattern
run_bench_pattern() {
    local pattern=$1

    info "Running benchmarks matching: $pattern"

    local cmd=$(build_test_cmd)
    if [[ -z "$TIMEOUT" ]]; then
        cmd="$cmd -timeout 30m"
    fi

    cmd="$cmd -bench=$pattern"

    if [[ "$SHOW_MEM" == "true" ]]; then
        cmd="$cmd -benchmem"
    fi

    cmd="$cmd ."

    cd "$SCRIPT_DIR"
    eval $cmd
    success "✓ Benchmarks completed"
}

# Run all parsing benchmarks
run_all_parse() {
    info "Running all parsing benchmarks"
    run_bench_pattern "^BenchmarkParse_.*_Schedules$"
}

# Run all matching benchmarks
run_all_match() {
    info "Running all matching benchmarks"
    warn "This may take several minutes..."
    run_bench_pattern "^BenchmarkMatch_"
}

# Run all benchmarks
run_all_bench() {
    info "Running ALL benchmarks"
    warn "This will take a long time (10+ minutes)..."
    run_bench_pattern "."
}

# Parse arguments
SHOW_MEM=false
VERBOSE=false
TIMEOUT=""
BENCH_MODE=false

if [[ $# -eq 0 ]]; then
    show_help
    exit 0
fi

while [[ $# -gt 0 ]]; do
    case $1 in
        --help|-h)
            show_help
            exit 0
            ;;
        --test)
            shift
            if [[ $# -gt 0 && ! $1 =~ ^-- ]]; then
                TEST_PATTERN=$1
                shift
            fi
            run_tests "$TEST_PATTERN"
            exit 0
            ;;
        --bench)
            BENCH_MODE=true
            shift
            ;;
        --parse)
            if [[ "$BENCH_MODE" != "true" ]]; then
                error "--parse requires --bench flag"
            fi
            shift
            if [[ $# -eq 0 ]]; then
                error "--parse requires a schedule count argument"
            fi
            run_parse_bench "$1"
            exit 0
            ;;
        --match)
            if [[ "$BENCH_MODE" != "true" ]]; then
                error "--match requires --bench flag"
            fi
            shift
            if [[ $# -lt 2 ]]; then
                error "--match requires two arguments: COUNT and WINDOW"
            fi
            run_match_bench "$1" "$2"
            exit 0
            ;;
        --pattern)
            if [[ "$BENCH_MODE" != "true" ]]; then
                error "--pattern requires --bench flag"
            fi
            shift
            if [[ $# -eq 0 ]]; then
                error "--pattern requires a pattern argument"
            fi
            run_bench_pattern "$1"
            exit 0
            ;;
        --all-parse)
            if [[ "$BENCH_MODE" != "true" ]]; then
                error "--all-parse requires --bench flag"
            fi
            shift
            run_all_parse
            exit 0
            ;;
        --all-match)
            if [[ "$BENCH_MODE" != "true" ]]; then
                error "--all-match requires --bench flag"
            fi
            shift
            run_all_match
            exit 0
            ;;
        --all)
            if [[ "$BENCH_MODE" != "true" ]]; then
                error "--all requires --bench flag"
            fi
            shift
            run_all_bench
            exit 0
            ;;
        --mem)
            SHOW_MEM=true
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
        *)
            error "Unknown option: $1. Use --help for usage information."
            ;;
    esac
done

# If we get here, show help
show_help
