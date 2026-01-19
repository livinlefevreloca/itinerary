package migrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Migration represents a database migration.
type Migration struct {
	Version       int
	Name          string
	UpSQL         string
	NoTransaction bool
	Dependencies  []int
}

var (
	filenameRegex    = regexp.MustCompile(`^(\d{3})_([a-zA-Z0-9_-]+)\.sql$`)
	upMarkerRegex    = regexp.MustCompile(`^--\s*\+migrate\s+Up(\s+notransaction)?\s*$`)
	dependsRegex     = regexp.MustCompile(`^--\s*\+migrate\s+Depends:\s*(.+)$`)
)

// ParseMigrationFile parses a single migration file and returns a Migration struct.
func ParseMigrationFile(path string) (*Migration, error) {
	// Parse filename
	filename := filepath.Base(path)
	matches := filenameRegex.FindStringSubmatch(filename)
	if matches == nil {
		return nil, fmt.Errorf("invalid migration filename format: %s (expected NNN_name.sql)", filename)
	}

	version, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("invalid version number in filename: %s", matches[1])
	}

	name := matches[2]

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read migration file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Find Up marker
	upMarkerFound := false
	noTransaction := false
	upMarkerLine := -1

	for i, line := range lines {
		if upMarkerRegex.MatchString(line) {
			upMarkerFound = true
			upMarkerLine = i
			matches := upMarkerRegex.FindStringSubmatch(line)
			if len(matches) > 1 && strings.TrimSpace(matches[1]) == "notransaction" {
				noTransaction = true
			}
			break
		}
	}

	if !upMarkerFound {
		return nil, fmt.Errorf("missing '-- +migrate Up' marker in migration file: %s", filename)
	}

	// Extract dependencies and SQL
	var dependencies []int
	sqlStartLine := upMarkerLine + 1

	// Look for dependency directives immediately after Up marker
	for i := upMarkerLine + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines and regular comments
		if line == "" || (strings.HasPrefix(line, "--") && !strings.Contains(line, "+migrate")) {
			continue
		}

		// Check for dependency directive
		if dependsRegex.MatchString(line) {
			matches := dependsRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				depsStr := strings.TrimSpace(matches[1])
				if depsStr == "" {
					return nil, fmt.Errorf("empty dependency list in migration file: %s", filename)
				}

				depParts := strings.Fields(depsStr)
				for _, depStr := range depParts {
					dep, err := strconv.Atoi(depStr)
					if err != nil {
						return nil, fmt.Errorf("invalid dependency version '%s' in migration file: %s", depStr, filename)
					}
					dependencies = append(dependencies, dep)
				}
			}
			sqlStartLine = i + 1
			continue
		}

		// If we hit a non-empty, non-comment, non-dependency line, SQL starts here
		if line != "" && !strings.HasPrefix(line, "--") {
			sqlStartLine = i
			break
		}

		sqlStartLine = i + 1
	}

	// Extract SQL content
	sqlLines := lines[sqlStartLine:]
	sql := strings.TrimSpace(strings.Join(sqlLines, "\n"))

	if sql == "" {
		return nil, fmt.Errorf("migration file contains no SQL statements: %s", filename)
	}

	return &Migration{
		Version:       version,
		Name:          name,
		UpSQL:         sql,
		NoTransaction: noTransaction,
		Dependencies:  dependencies,
	}, nil
}

// LoadMigrations loads all migrations from a directory, validates them, and returns them sorted by version.
func LoadMigrations(dir string) ([]Migration, error) {
	// Read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Parse all SQL files
	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .sql files that match migration pattern
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Check if filename matches migration pattern before parsing
		if !filenameRegex.MatchString(entry.Name()) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		migration, err := ParseMigrationFile(path)
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, *migration)
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	// Detect circular dependencies using DFS (check this before other validations)
	if err := detectCycle(migrations); err != nil {
		return nil, err
	}

	// Build version set for dependency validation
	versionSet := make(map[int]bool)
	for _, m := range migrations {
		versionSet[m.Version] = true
	}

	// Validate dependencies exist
	for _, m := range migrations {
		for _, dep := range m.Dependencies {
			if !versionSet[dep] {
				return nil, fmt.Errorf("migration %d depends on non-existent version %d", m.Version, dep)
			}
		}
	}

	// Validate sequence (no gaps, no duplicates)
	if len(migrations) > 0 {
		versionsSeen := make(map[int]bool)
		expectedVersion := 1

		for _, m := range migrations {
			// Check for duplicates
			if versionsSeen[m.Version] {
				return nil, fmt.Errorf("duplicate migration version: %d", m.Version)
			}
			versionsSeen[m.Version] = true

			// Check for gaps
			if m.Version != expectedVersion {
				return nil, fmt.Errorf("gap in migration versions: expected %d, found %d", expectedVersion, m.Version)
			}
			expectedVersion++
		}
	}

	return migrations, nil
}

// detectCycle uses a three-color DFS algorithm to detect circular dependencies.
// White (0) = unvisited, Gray (1) = visiting, Black (2) = completed
func detectCycle(migrations []Migration) error {
	// Build adjacency list
	graph := make(map[int][]int)
	for _, m := range migrations {
		graph[m.Version] = m.Dependencies
	}

	// Track colors for each node
	color := make(map[int]int)
	for _, m := range migrations {
		color[m.Version] = 0 // white
	}

	// DFS function
	var dfs func(int, []int) error
	dfs = func(node int, path []int) error {
		// Mark as visiting (gray)
		color[node] = 1
		path = append(path, node)

		// Visit all dependencies
		for _, dep := range graph[node] {
			if color[dep] == 1 {
				// Found a back edge (cycle)
				cyclePath := append(path, dep)
				return fmt.Errorf("circular dependency detected: %v", cyclePath)
			}
			if color[dep] == 0 {
				if err := dfs(dep, path); err != nil {
					return err
				}
			}
		}

		// Mark as completed (black)
		color[node] = 2
		return nil
	}

	// Run DFS from each unvisited node
	for _, m := range migrations {
		if color[m.Version] == 0 {
			if err := dfs(m.Version, []int{}); err != nil {
				return err
			}
		}
	}

	return nil
}
