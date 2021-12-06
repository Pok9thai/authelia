package schema

// ServerConfiguration represents the configuration of the http server.
type ServerConfiguration struct {
	Host               string `koanf:"host"`
	Port               int    `koanf:"port"`
	Path               string `koanf:"path"`
	AssetPath          string `koanf:"asset_path"`
	ReadBufferSize     int    `koanf:"read_buffer_size"`
	WriteBufferSize    int    `koanf:"write_buffer_size"`
	EnablePprof        bool   `koanf:"enable_endpoint_pprof"`
	EnableExpvars      bool   `koanf:"enable_endpoint_expvars"`
	DisableHealthcheck bool   `koanf:"disable_healthcheck"`

	TLS ServerTLSConfiguration `koanf:"tls"`
}

// ServerTLSConfiguration represents the configuration of the http servers TLS options.
type ServerTLSConfiguration struct {
	Certificate string `koanf:"certificate"`
	Key         string `koanf:"key"`
}

// DefaultServerConfiguration represents the default values of the ServerConfiguration.
var DefaultServerConfiguration = ServerConfiguration{
	Host:            "0.0.0.0",
	Port:            9091,
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}
