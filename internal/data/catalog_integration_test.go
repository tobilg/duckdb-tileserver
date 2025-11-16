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
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/tobilg/duckdb-tileserver/internal/conf"
)

// TestTableIncludesIntegration tests the complete flow from environment variable to table filtering
func TestTableIncludesIntegration(t *testing.T) {
	// Save original configuration
	originalConfig := conf.Configuration
	defer func() {
		conf.Configuration = originalConfig
		clearEnvVars()
	}()

	tests := []struct {
		name                string
		envTableIncludes    string
		envTableExcludes    string
		mockTables          []*Table
		expectedFilteredIDs []string
	}{
		{
			name:             "Include specific schema",
			envTableIncludes: "public",
			envTableExcludes: "",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.posts", Schema: "public", Table: "posts"},
				{ID: "private.secrets", Schema: "private", Table: "secrets"},
			},
			expectedFilteredIDs: []string{"public.users", "public.posts"},
		},
		{
			name:             "Include multiple schemas and tables",
			envTableIncludes: "public,admin.users",
			envTableExcludes: "",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.posts", Schema: "public", Table: "posts"},
				{ID: "admin.users", Schema: "admin", Table: "users"},
				{ID: "admin.logs", Schema: "admin", Table: "logs"},
				{ID: "private.secrets", Schema: "private", Table: "secrets"},
			},
			expectedFilteredIDs: []string{"public.users", "public.posts", "admin.users"},
		},
		{
			name:             "Include schema but exclude specific table",
			envTableIncludes: "public",
			envTableExcludes: "public.temp_logs",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.posts", Schema: "public", Table: "posts"},
				{ID: "public.temp_logs", Schema: "public", Table: "temp_logs"},
				{ID: "private.secrets", Schema: "private", Table: "secrets"},
			},
			expectedFilteredIDs: []string{"public.users", "public.posts"},
		},
		{
			name:             "No includes (all tables) but exclude some",
			envTableIncludes: "",
			envTableExcludes: "private,public.temp",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.temp", Schema: "public", Table: "temp"},
				{ID: "reports.monthly", Schema: "reports", Table: "monthly"},
				{ID: "private.secrets", Schema: "private", Table: "secrets"},
			},
			expectedFilteredIDs: []string{"public.users", "reports.monthly"},
		},
		{
			name:             "Case insensitive matching",
			envTableIncludes: "PUBLIC,Admin.USERS",
			envTableExcludes: "",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "Public.Posts", Schema: "Public", Table: "Posts"},
				{ID: "admin.users", Schema: "admin", Table: "users"},
				{ID: "Admin.LOGS", Schema: "Admin", Table: "LOGS"},
			},
			expectedFilteredIDs: []string{"public.users", "Public.Posts", "admin.users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment and reset viper
			clearEnvVars()
			viper.Reset()
			conf.Configuration = conf.Config{} // Clear the global configuration

			// Set environment variables
			if tt.envTableIncludes != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEINCLUDES", tt.envTableIncludes)
			}
			if tt.envTableExcludes != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEEXCLUDES", tt.envTableExcludes)
			}

			// Initialize configuration
			conf.InitConfig("", false)

			// Create catalog and set include/exclude
			catalog := &CatalogDB{}
			catalog.SetIncludeExclude(
				conf.Configuration.Database.TableIncludes,
				conf.Configuration.Database.TableExcludes,
			)

			// Test filtering
			var filteredIDs []string
			for _, table := range tt.mockTables {
				if catalog.isIncluded(table) {
					filteredIDs = append(filteredIDs, table.ID)
				}
			}

			testEquals(t, tt.expectedFilteredIDs, filteredIDs, "Filtered table IDs")
		})
	}
}

// TestTableIncludesWithConfigFile tests the integration with config files
func TestTableIncludesWithConfigFile(t *testing.T) {
	originalConfig := conf.Configuration
	defer func() {
		conf.Configuration = originalConfig
		clearEnvVars()
	}()

	// Create temporary config file
	configContent := `
[Database]
TableIncludes = ["config_schema", "config.table"]
TableExcludes = ["config_schema.excluded"]
`

	tempDir, err := os.MkdirTemp("", "duckdb-tileserver_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test_config.toml")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test with config file only
	clearEnvVars()
	viper.Reset()
	conf.InitConfig(configFile, false)

	catalog := &CatalogDB{}
	catalog.SetIncludeExclude(
		conf.Configuration.Database.TableIncludes,
		conf.Configuration.Database.TableExcludes,
	)

	mockTables := []*Table{
		{ID: "config_schema.table1", Schema: "config_schema", Table: "table1"},
		{ID: "config.table", Schema: "config", Table: "table"},
		{ID: "config_schema.excluded", Schema: "config_schema", Table: "excluded"},
		{ID: "other.table", Schema: "other", Table: "table"},
	}

	var filteredIDs []string
	for _, table := range mockTables {
		if catalog.isIncluded(table) {
			filteredIDs = append(filteredIDs, table.ID)
		}
	}

	expectedIDs := []string{"config_schema.table1", "config.table"}
	testEquals(t, expectedIDs, filteredIDs, "Config file filtering")

	// Test environment variable override
	os.Setenv("DUCKDBTS_DATABASE_TABLEINCLUDES", "override_schema")
	os.Setenv("DUCKDBTS_DATABASE_TABLEEXCLUDES", "override_schema.excluded")

	viper.Reset()
	conf.InitConfig(configFile, false)

	catalog = &CatalogDB{}
	catalog.SetIncludeExclude(
		conf.Configuration.Database.TableIncludes,
		conf.Configuration.Database.TableExcludes,
	)

	mockTables = []*Table{
		{ID: "override_schema.table1", Schema: "override_schema", Table: "table1"},
		{ID: "override_schema.excluded", Schema: "override_schema", Table: "excluded"},
		{ID: "config_schema.table1", Schema: "config_schema", Table: "table1"}, // From config, should not be included
		{ID: "other.table", Schema: "other", Table: "table"},
	}

	filteredIDs = []string{}
	for _, table := range mockTables {
		if catalog.isIncluded(table) {
			filteredIDs = append(filteredIDs, table.ID)
		}
	}

	expectedIDs = []string{"override_schema.table1"}
	testEquals(t, expectedIDs, filteredIDs, "Environment override filtering")
}

// TestTableIncludesRealWorldScenarios tests realistic usage scenarios
func TestTableIncludesRealWorldScenarios(t *testing.T) {
	originalConfig := conf.Configuration
	defer func() {
		conf.Configuration = originalConfig
		clearEnvVars()
	}()

	scenarios := []struct {
		name           string
		description    string
		envIncludes    string
		envExcludes    string
		tables         []*Table
		expectedTables []string
		explanation    string
	}{
		{
			name:        "Microservice database",
			description: "Include only user-related tables",
			envIncludes: "users,public.user_profiles,public.user_sessions",
			envExcludes: "",
			tables: []*Table{
				{ID: "users", Schema: "main", Table: "users"},
				{ID: "public.user_profiles", Schema: "public", Table: "user_profiles"},
				{ID: "public.user_sessions", Schema: "public", Table: "user_sessions"},
				{ID: "public.orders", Schema: "public", Table: "orders"},
				{ID: "internal.logs", Schema: "internal", Table: "logs"},
			},
			expectedTables: []string{"users", "public.user_profiles", "public.user_sessions"},
			explanation:    "Only user-related tables should be exposed",
		},
		{
			name:        "Multi-tenant application",
			description: "Include tenant schemas but exclude system tables",
			envIncludes: "tenant1,tenant2,shared",
			envExcludes: "tenant1.system_logs,tenant2.system_logs,shared.migrations",
			tables: []*Table{
				{ID: "tenant1.users", Schema: "tenant1", Table: "users"},
				{ID: "tenant1.system_logs", Schema: "tenant1", Table: "system_logs"},
				{ID: "tenant2.products", Schema: "tenant2", Table: "products"},
				{ID: "tenant2.system_logs", Schema: "tenant2", Table: "system_logs"},
				{ID: "shared.countries", Schema: "shared", Table: "countries"},
				{ID: "shared.migrations", Schema: "shared", Table: "migrations"},
				{ID: "admin.settings", Schema: "admin", Table: "settings"},
			},
			expectedTables: []string{
				"tenant1.users",
				"tenant2.products",
				"shared.countries",
			},
			explanation: "Tenant data accessible but system tables excluded",
		},
		{
			name:        "Development vs Production",
			description: "Include all but exclude test and temporary tables",
			envIncludes: "",
			envExcludes: "test,temp,public.temp_data,staging.test_results",
			tables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.products", Schema: "public", Table: "products"},
				{ID: "public.temp_data", Schema: "public", Table: "temp_data"},
				{ID: "test.scenarios", Schema: "test", Table: "scenarios"},
				{ID: "temp.calculations", Schema: "temp", Table: "calculations"},
				{ID: "staging.test_results", Schema: "staging", Table: "test_results"},
				{ID: "staging.reports", Schema: "staging", Table: "reports"},
			},
			expectedTables: []string{
				"public.users",
				"public.products",
				"staging.reports",
			},
			explanation: "Production tables accessible, test/temp excluded",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			clearEnvVars()
			viper.Reset()
			conf.Configuration = conf.Config{} // Clear the global configuration

			if scenario.envIncludes != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEINCLUDES", scenario.envIncludes)
			}
			if scenario.envExcludes != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEEXCLUDES", scenario.envExcludes)
			}

			conf.InitConfig("", false)

			catalog := &CatalogDB{}
			catalog.SetIncludeExclude(
				conf.Configuration.Database.TableIncludes,
				conf.Configuration.Database.TableExcludes,
			)

			var actualTables []string
			for _, table := range scenario.tables {
				if catalog.isIncluded(table) {
					actualTables = append(actualTables, table.ID)
				}
			}

			testEquals(t, scenario.expectedTables, actualTables, scenario.explanation)
		})
	}
}

// TestTableExcludesIntegration tests the complete flow from environment variable to table exclusion
func TestTableExcludesIntegration(t *testing.T) {
	// Save original configuration
	originalConfig := conf.Configuration
	defer func() {
		conf.Configuration = originalConfig
		clearEnvVars()
	}()

	tests := []struct {
		name                string
		envTableIncludes    string
		envTableExcludes    string
		mockTables          []*Table
		expectedFilteredIDs []string
	}{
		{
			name:             "Exclude specific schema",
			envTableIncludes: "",
			envTableExcludes: "private",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.posts", Schema: "public", Table: "posts"},
				{ID: "private.secrets", Schema: "private", Table: "secrets"},
				{ID: "private.config", Schema: "private", Table: "config"},
			},
			expectedFilteredIDs: []string{"public.users", "public.posts"},
		},
		{
			name:             "Exclude specific tables only",
			envTableIncludes: "",
			envTableExcludes: "public.temp_logs,admin.debug_info",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.temp_logs", Schema: "public", Table: "temp_logs"},
				{ID: "admin.settings", Schema: "admin", Table: "settings"},
				{ID: "admin.debug_info", Schema: "admin", Table: "debug_info"},
			},
			expectedFilteredIDs: []string{"public.users", "admin.settings"},
		},
		{
			name:             "Exclude multiple schemas",
			envTableIncludes: "",
			envTableExcludes: "temp,logs,staging",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "temp.calculations", Schema: "temp", Table: "calculations"},
				{ID: "logs.access", Schema: "logs", Table: "access"},
				{ID: "staging.test_data", Schema: "staging", Table: "test_data"},
				{ID: "production.data", Schema: "production", Table: "data"},
			},
			expectedFilteredIDs: []string{"public.users", "production.data"},
		},
		{
			name:             "Include specific schema but exclude some tables",
			envTableIncludes: "public,admin",
			envTableExcludes: "public.temp,admin.debug",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.temp", Schema: "public", Table: "temp"},
				{ID: "admin.settings", Schema: "admin", Table: "settings"},
				{ID: "admin.debug", Schema: "admin", Table: "debug"},
				{ID: "private.secrets", Schema: "private", Table: "secrets"},
			},
			expectedFilteredIDs: []string{"public.users", "admin.settings"},
		},
		{
			name:             "Complex exclusion scenario",
			envTableIncludes: "",
			envTableExcludes: "system,internal.migrations,public.cache,logs.debug",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.cache", Schema: "public", Table: "cache"},
				{ID: "system.config", Schema: "system", Table: "config"},
				{ID: "internal.migrations", Schema: "internal", Table: "migrations"},
				{ID: "internal.settings", Schema: "internal", Table: "settings"},
				{ID: "logs.access", Schema: "logs", Table: "access"},
				{ID: "logs.debug", Schema: "logs", Table: "debug"},
			},
			expectedFilteredIDs: []string{"public.users", "internal.settings", "logs.access"},
		},
		{
			name:             "Case insensitive exclusions",
			envTableIncludes: "",
			envTableExcludes: "TEMP,Private.SECRETS",
			mockTables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "temp.data", Schema: "temp", Table: "data"},
				{ID: "Temp.Cache", Schema: "Temp", Table: "Cache"},
				{ID: "private.secrets", Schema: "private", Table: "secrets"},
				{ID: "Private.SECRETS", Schema: "Private", Table: "SECRETS"},
			},
			expectedFilteredIDs: []string{"public.users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment and reset viper
			clearEnvVars()
			viper.Reset()
			conf.Configuration = conf.Config{} // Clear the global configuration

			// Set environment variables
			if tt.envTableIncludes != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEINCLUDES", tt.envTableIncludes)
			}
			if tt.envTableExcludes != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEEXCLUDES", tt.envTableExcludes)
			}

			// Initialize configuration
			conf.InitConfig("", false)

			// Create catalog and set include/exclude
			catalog := &CatalogDB{}
			catalog.SetIncludeExclude(
				conf.Configuration.Database.TableIncludes,
				conf.Configuration.Database.TableExcludes,
			)

			// Test filtering
			var filteredIDs []string
			for _, table := range tt.mockTables {
				if catalog.isIncluded(table) {
					filteredIDs = append(filteredIDs, table.ID)
				}
			}

			testEquals(t, tt.expectedFilteredIDs, filteredIDs, "Filtered table IDs")
		})
	}
}

// TestTableExcludesRealWorldScenarios tests realistic exclusion scenarios
func TestTableExcludesRealWorldScenarios(t *testing.T) {
	originalConfig := conf.Configuration
	defer func() {
		conf.Configuration = originalConfig
		clearEnvVars()
	}()

	scenarios := []struct {
		name           string
		description    string
		envIncludes    string
		envExcludes    string
		tables         []*Table
		expectedTables []string
		explanation    string
	}{
		{
			name:        "Security-focused exclusions",
			description: "Exclude all sensitive and system tables",
			envIncludes: "",
			envExcludes: "system,private,internal,auth.tokens,logs.sensitive",
			tables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.products", Schema: "public", Table: "products"},
				{ID: "system.config", Schema: "system", Table: "config"},
				{ID: "private.keys", Schema: "private", Table: "keys"},
				{ID: "internal.migrations", Schema: "internal", Table: "migrations"},
				{ID: "auth.tokens", Schema: "auth", Table: "tokens"},
				{ID: "auth.users", Schema: "auth", Table: "users"},
				{ID: "logs.access", Schema: "logs", Table: "access"},
				{ID: "logs.sensitive", Schema: "logs", Table: "sensitive"},
			},
			expectedTables: []string{"public.users", "public.products", "auth.users", "logs.access"},
			explanation:    "Only safe public and non-sensitive tables exposed",
		},
		{
			name:        "Development environment cleanup",
			description: "Exclude test, temp, and debug tables from production API",
			envIncludes: "",
			envExcludes: "test,temp,debug,staging.test_data,dev.playground",
			tables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.orders", Schema: "public", Table: "orders"},
				{ID: "test.scenarios", Schema: "test", Table: "scenarios"},
				{ID: "temp.calculations", Schema: "temp", Table: "calculations"},
				{ID: "debug.traces", Schema: "debug", Table: "traces"},
				{ID: "staging.test_data", Schema: "staging", Table: "test_data"},
				{ID: "staging.reports", Schema: "staging", Table: "reports"},
				{ID: "dev.playground", Schema: "dev", Table: "playground"},
				{ID: "dev.prototypes", Schema: "dev", Table: "prototypes"},
			},
			expectedTables: []string{"public.users", "public.orders", "staging.reports", "dev.prototypes"},
			explanation:    "Production tables accessible, development artifacts excluded",
		},
		{
			name:        "Performance-focused exclusions",
			description: "Exclude large log and archive tables to improve API performance",
			envIncludes: "",
			envExcludes: "logs,archive,backup,historical.raw_data",
			tables: []*Table{
				{ID: "public.users", Schema: "public", Table: "users"},
				{ID: "public.products", Schema: "public", Table: "products"},
				{ID: "logs.access", Schema: "logs", Table: "access"},
				{ID: "logs.error", Schema: "logs", Table: "error"},
				{ID: "archive.old_orders", Schema: "archive", Table: "old_orders"},
				{ID: "backup.user_snapshots", Schema: "backup", Table: "user_snapshots"},
				{ID: "historical.summary", Schema: "historical", Table: "summary"},
				{ID: "historical.raw_data", Schema: "historical", Table: "raw_data"},
			},
			expectedTables: []string{"public.users", "public.products", "historical.summary"},
			explanation:    "Core business tables accessible, heavy tables excluded",
		},
		{
			name:        "Multi-tenant with exclusions",
			description: "Include tenant schemas but exclude admin and system tables",
			envIncludes: "tenant1,tenant2,shared",
			envExcludes: "tenant1.admin_config,tenant2.system_logs,shared.migrations",
			tables: []*Table{
				{ID: "tenant1.users", Schema: "tenant1", Table: "users"},
				{ID: "tenant1.admin_config", Schema: "tenant1", Table: "admin_config"},
				{ID: "tenant2.products", Schema: "tenant2", Table: "products"},
				{ID: "tenant2.system_logs", Schema: "tenant2", Table: "system_logs"},
				{ID: "shared.countries", Schema: "shared", Table: "countries"},
				{ID: "shared.migrations", Schema: "shared", Table: "migrations"},
				{ID: "admin.global_settings", Schema: "admin", Table: "global_settings"},
			},
			expectedTables: []string{"tenant1.users", "tenant2.products", "shared.countries"},
			explanation:    "Tenant data accessible but admin/system tables excluded",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			clearEnvVars()
			viper.Reset()
			conf.Configuration = conf.Config{} // Clear the global configuration

			if scenario.envIncludes != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEINCLUDES", scenario.envIncludes)
			}
			if scenario.envExcludes != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEEXCLUDES", scenario.envExcludes)
			}

			conf.InitConfig("", false)

			catalog := &CatalogDB{}
			catalog.SetIncludeExclude(
				conf.Configuration.Database.TableIncludes,
				conf.Configuration.Database.TableExcludes,
			)

			var actualTables []string
			for _, table := range scenario.tables {
				if catalog.isIncluded(table) {
					actualTables = append(actualTables, table.ID)
				}
			}

			testEquals(t, scenario.expectedTables, actualTables, scenario.explanation)
		})
	}
}

// Helper function to clear environment variables
func clearEnvVars() {
	envVars := []string{
		"DUCKDBTS_DATABASE_TABLEINCLUDES",
		"DUCKDBTS_DATABASE_TABLEEXCLUDES",
		"DUCKDBTS_DATABASE_PATH",
		"DUCKDBTS_DATABASE_TABLENAME",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}
