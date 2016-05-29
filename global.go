package grinklers

import (
	"github.com/Sirupsen/logrus"
)

var Logger = logrus.New()

func init() {
	Logger.Level = logrus.DebugLevel
}
