package data

/*
 Copyright 2019 - 2025 Crunchy Data Solutions, Inc.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

import (
	"fmt"
	"testing"
)

// TestTableIncludeExcludeLogic tests the table filtering logic
func TestTableIncludeExcludeLogic(t *testing.T) {
	tests := []struct {
		name        string
		table       *Table
		includes    []string
		excludes    []string
		shouldMatch bool
	}{
		{
			name: "No includes, no excludes - should include",
			table: &Table{
				ID:     "public.users",
				Schema: "public",
				Table:  "users",
			},
			includes:    []string{},
			excludes:    []string{},
			shouldMatch: true,
		},
		{
			name: "Schema in includes - should include",
			table: &Table{
				ID:     "public.users",
				Schema: "public",
				Table:  "users",
			},
			includes:    []string{"public"},
			excludes:    []string{},
			shouldMatch: true,
		},
		{
			name: "Table ID in includes - should include",
			table: &Table{
				ID:     "public.users",
				Schema: "public",
				Table:  "users",
			},
			includes:    []string{"public.users"},
			excludes:    []string{},
			shouldMatch: true,
		},
		{
			name: "Neither schema nor table ID in includes - should exclude",
			table: &Table{
				ID:     "public.users",
				Schema: "public",
				Table:  "users",
			},
			includes:    []string{"private"},
			excludes:    []string{},
			shouldMatch: false,
		},
		{
			name: "Schema in excludes - should exclude",
			table: &Table{
				ID:     "private.secrets",
				Schema: "private",
				Table:  "secrets",
			},
			includes:    []string{},
			excludes:    []string{"private"},
			shouldMatch: false,
		},
		{
			name: "Table ID in excludes - should exclude",
			table: &Table{
				ID:     "public.temp_table",
				Schema: "public",
				Table:  "temp_table",
			},
			includes:    []string{},
			excludes:    []string{"public.temp_table"},
			shouldMatch: false,
		},
		{
			name: "In includes but also in excludes - should exclude (excludes take precedence)",
			table: &Table{
				ID:     "public.users",
				Schema: "public",
				Table:  "users",
			},
			includes:    []string{"public"},
			excludes:    []string{"public.users"},
			shouldMatch: false,
		},
		{
			name: "Case insensitive matching - schema",
			table: &Table{
				ID:     "Public.Users",
				Schema: "Public",
				Table:  "Users",
			},
			includes:    []string{"public"},
			excludes:    []string{},
			shouldMatch: true,
		},
		{
			name: "Case insensitive matching - table ID",
			table: &Table{
				ID:     "Public.Users",
				Schema: "Public",
				Table:  "Users",
			},
			includes:    []string{"public.users"},
			excludes:    []string{},
			shouldMatch: true,
		},
		{
			name: "Multiple includes, matches one",
			table: &Table{
				ID:     "schema1.table1",
				Schema: "schema1",
				Table:  "table1",
			},
			includes:    []string{"schema2", "schema1.table1", "schema3"},
			excludes:    []string{},
			shouldMatch: true,
		},
		{
			name: "Multiple includes, matches none",
			table: &Table{
				ID:     "schema1.table1",
				Schema: "schema1",
				Table:  "table1",
			},
			includes:    []string{"schema2", "schema3.table1", "schema4"},
			excludes:    []string{},
			shouldMatch: false,
		},
		{
			name: "Table name without schema in includes",
			table: &Table{
				ID:     "users",
				Schema: "main",
				Table:  "users",
			},
			includes:    []string{"users"},
			excludes:    []string{},
			shouldMatch: true,
		},
		{
			name: "Schema-qualified table in includes, simple table name in table",
			table: &Table{
				ID:     "users",
				Schema: "main",
				Table:  "users",
			},
			includes:    []string{"main.users"},
			excludes:    []string{},
			shouldMatch: false, // Should not match because table ID is "users", not "main.users"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := &CatalogDB{}
			catalog.SetIncludeExclude(tt.includes, tt.excludes)

			result := catalog.isIncluded(tt.table)
			testEquals(t, tt.shouldMatch, result, fmt.Sprintf("Table filtering for %s", tt.table.ID))
		})
	}
}

// TestSetIncludeExclude tests the SetIncludeExclude method
func TestSetIncludeExclude(t *testing.T) {
	catalog := &CatalogDB{}

	includes := []string{"Public", "Schema1.Table1", "mixed_Case"}
	excludes := []string{"Private", "Schema2.TABLE2"}

	catalog.SetIncludeExclude(includes, excludes)

	// Check that all entries are stored in lowercase
	expectedIncludes := map[string]string{
		"public":         "public",
		"schema1.table1": "schema1.table1",
		"mixed_case":     "mixed_case",
	}

	expectedExcludes := map[string]string{
		"private":        "private",
		"schema2.table2": "schema2.table2",
	}

	testEquals(t, expectedIncludes, catalog.tableIncludes, "Table includes")
	testEquals(t, expectedExcludes, catalog.tableExcludes, "Table excludes")
}

// TestIsMatchSchemaTable tests the isMatchSchemaTable function directly
func TestIsMatchSchemaTable(t *testing.T) {
	tests := []struct {
		name        string
		table       *Table
		list        map[string]string
		shouldMatch bool
	}{
		{
			name: "Schema matches",
			table: &Table{
				ID:     "public.users",
				Schema: "public",
				Table:  "users",
			},
			list: map[string]string{
				"public": "public",
			},
			shouldMatch: true,
		},
		{
			name: "Table ID matches",
			table: &Table{
				ID:     "public.users",
				Schema: "public",
				Table:  "users",
			},
			list: map[string]string{
				"public.users": "public.users",
			},
			shouldMatch: true,
		},
		{
			name: "No match",
			table: &Table{
				ID:     "public.users",
				Schema: "public",
				Table:  "users",
			},
			list: map[string]string{
				"private":      "private",
				"public.posts": "public.posts",
			},
			shouldMatch: false,
		},
		{
			name: "Case insensitive schema match",
			table: &Table{
				ID:     "Public.Users",
				Schema: "Public",
				Table:  "Users",
			},
			list: map[string]string{
				"public": "public",
			},
			shouldMatch: true,
		},
		{
			name: "Case insensitive table ID match",
			table: &Table{
				ID:     "Public.Users",
				Schema: "Public",
				Table:  "Users",
			},
			list: map[string]string{
				"public.users": "public.users",
			},
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMatchSchemaTable(tt.table, tt.list)
			testEquals(t, tt.shouldMatch, result, fmt.Sprintf("Schema/table matching for %s", tt.table.ID))
		})
	}
}

// TestEmptyIncludesExcludes tests behavior with empty includes/excludes
func TestEmptyIncludesExcludes(t *testing.T) {
	catalog := &CatalogDB{}

	// Test with nil slices
	catalog.SetIncludeExclude(nil, nil)

	table := &Table{
		ID:     "any.table",
		Schema: "any",
		Table:  "table",
	}

	// Should include when no restrictions
	result := catalog.isIncluded(table)
	testEquals(t, true, result, "Should include when no restrictions")

	// Test with empty slices
	catalog.SetIncludeExclude([]string{}, []string{})
	result = catalog.isIncluded(table)
	testEquals(t, true, result, "Should include with empty slices")
}

// TestComplexIncludeExcludeScenarios tests complex real-world scenarios
func TestComplexIncludeExcludeScenarios(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		excludes []string
		tables   []struct {
			table    *Table
			expected bool
		}
	}{
		{
			name:     "Include specific schema, exclude specific table",
			includes: []string{"public"},
			excludes: []string{"public.temp_logs"},
			tables: []struct {
				table    *Table
				expected bool
			}{
				{
					table: &Table{
						ID:     "public.users",
						Schema: "public",
						Table:  "users",
					},
					expected: true,
				},
				{
					table: &Table{
						ID:     "public.temp_logs",
						Schema: "public",
						Table:  "temp_logs",
					},
					expected: false, // Excluded
				},
				{
					table: &Table{
						ID:     "private.secrets",
						Schema: "private",
						Table:  "secrets",
					},
					expected: false, // Not in includes
				},
			},
		},
		{
			name:     "Multiple schemas, multiple excludes",
			includes: []string{"public", "reports"},
			excludes: []string{"public.temp", "reports.legacy"},
			tables: []struct {
				table    *Table
				expected bool
			}{
				{
					table: &Table{
						ID:     "public.users",
						Schema: "public",
						Table:  "users",
					},
					expected: true,
				},
				{
					table: &Table{
						ID:     "reports.monthly",
						Schema: "reports",
						Table:  "monthly",
					},
					expected: true,
				},
				{
					table: &Table{
						ID:     "public.temp",
						Schema: "public",
						Table:  "temp",
					},
					expected: false, // Excluded
				},
				{
					table: &Table{
						ID:     "reports.legacy",
						Schema: "reports",
						Table:  "legacy",
					},
					expected: false, // Excluded
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := &CatalogDB{}
			catalog.SetIncludeExclude(tt.includes, tt.excludes)

			for i, tableTest := range tt.tables {
				result := catalog.isIncluded(tableTest.table)
				testEquals(t, tableTest.expected, result,
					fmt.Sprintf("Table %d (%s) filtering", i, tableTest.table.ID))
			}
		})
	}
}

// TestTableExcludesOnlyScenarios tests scenarios where only excludes are specified (no includes)
func TestTableExcludesOnlyScenarios(t *testing.T) {
	tests := []struct {
		name     string
		excludes []string
		tables   []struct {
			table    *Table
			expected bool
		}
	}{
		{
			name:     "Exclude sensitive schemas",
			excludes: []string{"private", "system", "internal"},
			tables: []struct {
				table    *Table
				expected bool
			}{
				{
					table: &Table{
						ID:     "public.users",
						Schema: "public",
						Table:  "users",
					},
					expected: true, // Not excluded
				},
				{
					table: &Table{
						ID:     "private.secrets",
						Schema: "private",
						Table:  "secrets",
					},
					expected: false, // Excluded by schema
				},
				{
					table: &Table{
						ID:     "system.config",
						Schema: "system",
						Table:  "config",
					},
					expected: false, // Excluded by schema
				},
				{
					table: &Table{
						ID:     "internal.migrations",
						Schema: "internal",
						Table:  "migrations",
					},
					expected: false, // Excluded by schema
				},
				{
					table: &Table{
						ID:     "reports.monthly",
						Schema: "reports",
						Table:  "monthly",
					},
					expected: true, // Not excluded
				},
			},
		},
		{
			name:     "Exclude specific problematic tables",
			excludes: []string{"public.temp_cache", "logs.debug_info", "staging.test_data"},
			tables: []struct {
				table    *Table
				expected bool
			}{
				{
					table: &Table{
						ID:     "public.users",
						Schema: "public",
						Table:  "users",
					},
					expected: true, // Not excluded
				},
				{
					table: &Table{
						ID:     "public.temp_cache",
						Schema: "public",
						Table:  "temp_cache",
					},
					expected: false, // Excluded by table ID
				},
				{
					table: &Table{
						ID:     "logs.access",
						Schema: "logs",
						Table:  "access",
					},
					expected: true, // Not excluded
				},
				{
					table: &Table{
						ID:     "logs.debug_info",
						Schema: "logs",
						Table:  "debug_info",
					},
					expected: false, // Excluded by table ID
				},
				{
					table: &Table{
						ID:     "staging.test_data",
						Schema: "staging",
						Table:  "test_data",
					},
					expected: false, // Excluded by table ID
				},
				{
					table: &Table{
						ID:     "staging.reports",
						Schema: "staging",
						Table:  "reports",
					},
					expected: true, // Not excluded
				},
			},
		},
		{
			name:     "Mixed schema and table exclusions",
			excludes: []string{"temp", "debug.traces", "private.keys", "logs"},
			tables: []struct {
				table    *Table
				expected bool
			}{
				{
					table: &Table{
						ID:     "public.users",
						Schema: "public",
						Table:  "users",
					},
					expected: true, // Not excluded
				},
				{
					table: &Table{
						ID:     "temp.calculations",
						Schema: "temp",
						Table:  "calculations",
					},
					expected: false, // Excluded by schema
				},
				{
					table: &Table{
						ID:     "debug.traces",
						Schema: "debug",
						Table:  "traces",
					},
					expected: false, // Excluded by table ID
				},
				{
					table: &Table{
						ID:     "debug.performance",
						Schema: "debug",
						Table:  "performance",
					},
					expected: true, // Not excluded (only debug.traces is excluded)
				},
				{
					table: &Table{
						ID:     "private.keys",
						Schema: "private",
						Table:  "keys",
					},
					expected: false, // Excluded by table ID
				},
				{
					table: &Table{
						ID:     "private.settings",
						Schema: "private",
						Table:  "settings",
					},
					expected: true, // Not excluded (only private.keys is excluded)
				},
				{
					table: &Table{
						ID:     "logs.access",
						Schema: "logs",
						Table:  "access",
					},
					expected: false, // Excluded by schema
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := &CatalogDB{}
			catalog.SetIncludeExclude([]string{}, tt.excludes) // No includes, only excludes

			for i, tableTest := range tt.tables {
				result := catalog.isIncluded(tableTest.table)
				testEquals(t, tableTest.expected, result,
					fmt.Sprintf("Table %d (%s) exclusion filtering", i, tableTest.table.ID))
			}
		})
	}
}
