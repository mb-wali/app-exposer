package common

import (
	"github.com/sirupsen/logrus"
)

// Log contains the default logger to use.
var Log = logrus.WithFields(logrus.Fields{
	"service": "app-exposer",
	"art-id":  "app-exposer",
	"group":   "org.cyverse",
})
