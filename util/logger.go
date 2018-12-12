package util

import (
	"os"

	"github.com/Sirupsen/logrus"
)

func InitLogLevel() {
	level := os.Getenv("LOG_LEVEL")
	if level != "" {
		lvl, err := logrus.ParseLevel(level)
		if err == nil {
			logrus.SetLevel(lvl)
		}
	}
}

// Logger is global logger for the application
var Logger = logrus.New()
