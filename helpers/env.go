package helpers

import "os"

func override(val interface{}, env string) interface{} {
	envVar := os.Getenv(env)

	switch val.(type) {
	case string:
		if envVar != "" {
			val = envVar
			Logger.Debug("%s = %s", env, val.(string))
		}

	case bool:
		if envVar != "" {
			val = map[string]bool{
				"true": true,
			}[envVar]
			Logger.Debug("%s = %s", env, val.(bool))
		}
	}

	return val
}

var (
	PortProfiler   string
	ProfilerEnable bool

	PrintLogMessages bool
	PortRcon         string
	PortRPC          string
	PortHelen        string
	PortMQ           string
	LogsTFAPIKey     string
)

func initConstants() {
	PortProfiler = override("6061", "PROFILER_PORT").(string)
	ProfilerEnable = override(false, "PROFILER_ENABLE").(bool)

	PrintLogMessages = override(false, "PRINT_LOG_MESSAGES").(bool)
	PortRcon = override("8002", "RCON_PORT").(string)
	PortRPC = override("8001", "PAULING_PORT").(string)
	PortHelen = override("8081", "HELEN_PORT").(string)
	LogsTFAPIKey = override("", "LOGSTF_KEY").(string)
	PortMQ = override("", "MQ_PORT").(string)
}
