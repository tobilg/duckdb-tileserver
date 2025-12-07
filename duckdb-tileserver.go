package main

/*
# Running
Usage: ./duckdb-tileserver [ -test ] [ --database-path /path/to/database.db ]

Browser: e.g. http://localhost:9000/

# Configuration
DuckDB file path in env var `DUCKDBTS_DATABASE_PATH`
Example: `export DUCKDBTS_DATABASE_PATH="/path/to/database.db"`

Table filtering via env vars `DUCKDBTS_DATABASE_TABLEINCLUDES` and `DUCKDBTS_DATABASE_TABLEEXCLUDES` (optional)
Examples:
  `export DUCKDBTS_DATABASE_TABLEINCLUDES="buildings,roads"`
  `export DUCKDBTS_DATABASE_TABLEEXCLUDES="temp,staging"`
If not specified, all tables with geometry columns will be served as MVT tile layers

# Logging
Logging to stdout
*/

import (
	"fmt"
	"os"

	"github.com/tobilg/duckdb-tileserver/internal/conf"
	"github.com/tobilg/duckdb-tileserver/internal/data"
	"github.com/tobilg/duckdb-tileserver/internal/service"
	"github.com/tobilg/duckdb-tileserver/internal/ui"

	"github.com/pborman/getopt/v2"
	log "github.com/sirupsen/logrus"
)

var flagTestModeOn bool
var flagDebugOn bool
var flagDevModeOn bool
var flagHelp bool
var flagVersion bool
var flagConfigFilename string
var flagDuckDBPath string

var flagDisableUi bool

func init() {
	initCommnandOptions()
}

func initCommnandOptions() {
	getopt.FlagLong(&flagHelp, "help", '?', "Show command usage")
	getopt.FlagLong(&flagConfigFilename, "config", 'c', "", "config file name")
	getopt.FlagLong(&flagDebugOn, "debug", 'd', "Set logging level to TRACE")
	getopt.FlagLong(&flagDevModeOn, "devel", 0, "Run in development mode")
	getopt.FlagLong(&flagTestModeOn, "test", 't', "Serve mock data for testing")
	getopt.FlagLong(&flagVersion, "version", 'v', "Output the version information")
	getopt.FlagLong(&flagDuckDBPath, "database-path", 0, "", "Path to DuckDB database file")
	getopt.FlagLong(&flagDisableUi, "disable-ui", 0, "Disable HTML UI routes")
}

func main() {
	getopt.Parse()

	if flagHelp {
		getopt.Usage()
		os.Exit(1)
	}

	if flagVersion {
		fmt.Printf("%s %s\n", conf.AppConfig.Name, conf.AppConfig.Version)
		os.Exit(1)
	}

	log.Infof("----  %s - Version %s ----------\n", conf.AppConfig.Name, conf.AppConfig.Version)

	conf.InitConfig(flagConfigFilename, flagDebugOn)

	// Set DuckDB parameters from command line if provided
	if flagDuckDBPath != "" {
		conf.Configuration.Database.DatabasePath = flagDuckDBPath
	}

	// Set UI disable flag from command line
	if flagDisableUi {
		conf.Configuration.Server.DisableUi = true
	}

	if flagTestModeOn || flagDevModeOn {
		ui.HTMLDynamicLoad = true
		log.Info("Running in development mode")
	}
	// Commandline over-rides config file for debugging
	if flagDebugOn || conf.Configuration.Server.Debug {
		log.SetLevel(log.TraceLevel)
		log.Debugf("Log level = DEBUG\n")
	}
	conf.DumpConfig()

	//-- Initialize catalog (with DB conn if used)
	var catalog data.Catalog
	if flagTestModeOn {
		catalog = data.CatMockInstance()
	} else {
		catalog = data.CatDBInstance()
	}
	includes := conf.Configuration.Database.TableIncludes
	excludes := conf.Configuration.Database.TableExcludes
	catalog.SetIncludeExclude(includes, excludes)

	//-- Start up service
	service.Initialize()
	service.Serve(catalog)
}
