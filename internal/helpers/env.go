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

	DBHost     = override("127.0.0.1", "DATABASE_HOST").(string)
	DBPort     = override("5432", "DATABASE_PORT").(string)
	DBName     = override("tf2stadium", "DATABASE_NAME").(string)
	DBUser     = override("tf2stadium", "DATABASE_USER").(string)
	DBPassword = override("dickbutt", "DATABASE_PASSWORD").(string)
)
