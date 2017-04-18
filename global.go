package grinklers

import (
	"github.com/Sirupsen/logrus"
)

// Logger is global logger for the application
var Logger = logrus.New()

func init() {
	Logger.Level = logrus.DebugLevel
}
