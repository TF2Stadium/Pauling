package helpers

import "os"

func override(val interface{}, env string) interface{} {
	envVar := os.Getenv(env)

	switch val.(type) {
	case string:
		if envVar != "" {
			val = envVar
		}

	case bool:
		if envVar != "" {
			val = map[string]bool{
				"true": true,
			}[envVar]
		}
	}

	return val
}

var (
	PortProfiler   = override("6061", "PROFILER_PORT").(string)
	ProfilerEnable = override(false, "PROFILER_ENABLE").(bool)

	PrintLogMessages = override(false, "PRINT_LOG_MESSAGES").(bool)
	PortRcon         = override("8002", "RCON_PORT").(string)
	PortRPC          = override("8001", "PAULING_PORT").(string)
	PortHelen        = override("8081", "HELEN_PORT").(string)
)
