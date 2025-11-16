package conf

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
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/spf13/viper"
)

// TestTableIncludesEnvironmentVariable tests that TableIncludes can be set via environment variable
func TestTableIncludesEnvironmentVariable(t *testing.T) {
	// Clear any existing environment variables
	defer clearConfigEnvVars()

	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "Single table",
			envValue: "public.table1",
			expected: []string{"public.table1"},
		},
		{
			name:     "Multiple tables",
			envValue: "public,schema1.table1,table2",
			expected: []string{"public", "schema1.table1", "table2"},
		},
		{
			name:     "Empty value",
			envValue: "",
			expected: []string{},
		},
		{
			name:     "Schema only",
			envValue: "public",
			expected: []string{"public"},
		},
		{
			name:     "Complex table names",
			envValue: "my_schema.my_table,another_schema.complex_table_name",
			expected: []string{"my_schema.my_table", "another_schema.complex_table_name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all environment variables first
			clearConfigEnvVars()

			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEINCLUDES", tt.envValue)
			}

			// Reset viper for clean state
			viper.Reset()

			// Initialize config
			InitConfig("", false)

			// Check result
			equals(t, tt.expected, Configuration.Database.TableIncludes, "TableIncludes")

			// Clean up
			clearConfigEnvVars()
		})
	}
}

// TestTableExcludesEnvironmentVariable tests that TableExcludes can be set via environment variable
func TestTableExcludesEnvironmentVariable(t *testing.T) {
	// Clear any existing environment variables
	defer clearConfigEnvVars()

	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "Single table exclusion",
			envValue: "private.secrets",
			expected: []string{"private.secrets"},
		},
		{
			name:     "Multiple table exclusions",
			envValue: "private,temp,logs.debug",
			expected: []string{"private", "temp", "logs.debug"},
		},
		{
			name:     "Empty value",
			envValue: "",
			expected: []string{},
		},
		{
			name:     "Schema exclusion",
			envValue: "temp",
			expected: []string{"temp"},
		},
		{
			name:     "Complex exclusion patterns",
			envValue: "temp_schema,staging.test_data,logs.debug_logs",
			expected: []string{"temp_schema", "staging.test_data", "logs.debug_logs"},
		},
		{
			name:     "System and private exclusions",
			envValue: "system,private,internal.migrations",
			expected: []string{"system", "private", "internal.migrations"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all environment variables first
			clearConfigEnvVars()

			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("DUCKDBTS_DATABASE_TABLEEXCLUDES", tt.envValue)
			}

			// Reset viper for clean state
			viper.Reset()

			// Initialize config
			InitConfig("", false)

			// Check result
			equals(t, tt.expected, Configuration.Database.TableExcludes, "TableExcludes")

			// Clean up
			clearConfigEnvVars()
		})
	}
}

// TestConfigFileOverriddenByEnvironment tests that environment variables take precedence over config file
func TestConfigFileOverriddenByEnvironment(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	// Create a temporary config file
	configContent := `
[Database]
TableIncludes = ["file_table1", "file_table2"]
TableExcludes = ["file_exclude"]
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

	// Set environment variables that should override config file
	os.Setenv("DUCKDBTS_DATABASE_TABLEINCLUDES", "env_table1,env_table2")
	os.Setenv("DUCKDBTS_DATABASE_TABLEEXCLUDES", "env_exclude")
	defer func() {
		os.Unsetenv("DUCKDBTS_DATABASE_TABLEINCLUDES")
		os.Unsetenv("DUCKDBTS_DATABASE_TABLEEXCLUDES")
	}()

	viper.Reset()
	InitConfig(configFile, false)

	// Environment variables should take precedence
	expectedIncludes := []string{"env_table1", "env_table2"}
	expectedExcludes := []string{"env_exclude"}

	equals(t, expectedIncludes, Configuration.Database.TableIncludes, "TableIncludes from env")
	equals(t, expectedExcludes, Configuration.Database.TableExcludes, "TableExcludes from env")
}

// TestConfigFileOnly tests that config file values are used when no environment variables are set
func TestConfigFileOnly(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	configContent := `
[Database]
TableIncludes = ["config_table1", "config_table2"]
TableExcludes = ["config_exclude"]
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

	viper.Reset()
	InitConfig(configFile, false)

	expectedIncludes := []string{"config_table1", "config_table2"}
	expectedExcludes := []string{"config_exclude"}

	equals(t, expectedIncludes, Configuration.Database.TableIncludes, "TableIncludes from config")
	equals(t, expectedExcludes, Configuration.Database.TableExcludes, "TableExcludes from config")
}

// TestDefaultValues tests that default values are used when no config file or environment variables are set
func TestDefaultValues(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	viper.Reset()
	InitConfig("", false)

	// Should have empty slices as defaults
	equals(t, []string{}, Configuration.Database.TableIncludes, "Default TableIncludes")
	equals(t, []string{}, Configuration.Database.TableExcludes, "Default TableExcludes")
}

// TestEnvironmentVariableFormat tests various formats for the environment variable
func TestEnvironmentVariableFormat(t *testing.T) {
	clearConfigEnvVars()
	defer clearConfigEnvVars()

	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "No spaces",
			envValue: "table1,table2,table3",
			expected: []string{"table1", "table2", "table3"},
		},
		{
			name:     "With spaces (Viper doesn't trim)",
			envValue: "table1, table2 , table3",
			expected: []string{"table1", " table2 ", " table3"},
		},
		{
			name:     "Single item",
			envValue: "single_table",
			expected: []string{"single_table"},
		},
		{
			name:     "Mixed schema.table and table only",
			envValue: "schema1.table1,table2,schema2.table3",
			expected: []string{"schema1.table1", "table2", "schema2.table3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnvVars()
			os.Setenv("DUCKDBTS_DATABASE_TABLEINCLUDES", tt.envValue)

			viper.Reset()
			InitConfig("", false)

			// Check that configuration matches expected values
			equals(t, tt.expected, Configuration.Database.TableIncludes, "TableIncludes")
		})
	}
}

// Helper function to clear all configuration-related environment variables
func clearConfigEnvVars() {
	envVars := []string{
		"DUCKDBTS_DATABASE_TABLEINCLUDES",
		"DUCKDBTS_DATABASE_TABLEEXCLUDES",
		"DUCKDBTS_DATABASE_PATH",
		"DUCKDBTS_DATABASE_TABLENAME",
		"DUCKDBTS_SERVER_HTTPPORT",
		"DUCKDBTS_SERVER_DEBUG",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}

	// Also clear the global Configuration variable
	Configuration = Config{}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}, msg string) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("%s:%d: %s - expected: %#v; got: %#v\n", filepath.Base(file), line, msg, exp, act)
		tb.FailNow()
	}
}
