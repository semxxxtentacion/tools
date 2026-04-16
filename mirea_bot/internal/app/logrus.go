package app

import (
	"github.com/sirupsen/logrus"
	"mirea-qr/internal/config"
)

func NewLogger(cfg config.Config) *logrus.Logger {
	log := logrus.New()

	if cfg.Debug {
		log.SetLevel(logrus.TraceLevel)
	} else {
		log.SetLevel(logrus.ErrorLevel)
	}
	log.SetFormatter(&logrus.TextFormatter{})

	return log
}
