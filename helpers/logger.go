package helpers

import (
	"log/syslog"
	"os"

	"github.com/op/go-logging"
)

var Logger = logging.MustGetLogger("main")

var format = logging.MustStringFormatter(
	`%{time:15:04:05} %{color} [%{level:.4s}] %{shortfile} %{shortfunc}() : %{message} %{color:reset}`,
)

func initLogger() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
	if addr := os.Getenv("PAPERTRAIL_ADDR"); addr != "" {
		writer, err := syslog.Dial("udp4", addr, syslog.LOG_EMERG, "Pauling")
		if err != nil {
			Logger.Fatal(err.Error())
		}

		format = logging.MustStringFormatter(`[%{level:.4s}] %{shortfile} %{shortfunc}() : %{message}`)
		syslogBackend := logging.NewBackendFormatter(&logging.SyslogBackend{Writer: writer}, format)
		logging.SetBackend(backendFormatter, syslogBackend)
	} else {
		logging.SetBackend(backendFormatter)
	}
}
