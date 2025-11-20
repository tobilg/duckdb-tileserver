package conf

var setVersion string = "0.1.2"

// AppConfiguration is the set of global application configuration constants.
type AppConfiguration struct {
	// AppName name of the software
	Name string
	// AppVersion version number of the software
	Version   string
	EnvPrefix string
}

var AppConfig = AppConfiguration{
	Name:      "duckdb-tileserver",
	Version:   setVersion,
	EnvPrefix: "DUCKDBTS",
}
