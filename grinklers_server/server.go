package main

import (
	"os"
	"os/signal"
	"sync"

	c "git.amikhalev.com/amikhalev/grinklers/config"
	l "git.amikhalev.com/amikhalev/grinklers/logic"
	"git.amikhalev.com/amikhalev/grinklers/mqtt"
	"git.amikhalev.com/amikhalev/grinklers/util"
	log "github.com/Sirupsen/logrus"
	"github.com/joho/godotenv"
)

func main() {
	util.Logger.Level = log.DebugLevel
	var logger = util.Logger.WithField("module", "server")
	// channel which is notified on an interrupt signal
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	godotenv.Load()

	config, err := c.LoadConfig()
	if err != nil {
		logger.WithError(err).Fatalf("error loading config")
	}

	logger.Info("writing back config")
	c.WriteConfig(&config)

	err = config.SectionInterface.Initialize()
	if err != nil {
		logger.WithError(err).Fatalf("error initializing sections")
	}

	waitGroup := sync.WaitGroup{}

	secRunner := l.NewSectionRunner(config.SectionInterface)
	secRunner.Start(&waitGroup)

	sections := config.Sections
	programs := config.Programs

	updater := mqtt.NewMQTTUpdater(&config, secRunner)

	logger.Debug("initializing sections and programs")

	stopAll := func() {
		for i := range sections {
			sections[i].SetState(false, config.SectionInterface)
		}
	}
	stopAll()

	for i := range programs {
		programs[i].Start(secRunner, &waitGroup)
	}

	logger.WithFields(log.Fields{
		"lenSections": len(sections), "lenPrograms": len(programs),
	}).Info("initialized sections and programs")

	api := mqtt.NewMQTTApi(&config, secRunner)
	api.Start()

	updater.Start(api)

	<-sigc

	logger.Info("cleaning up...")
	updater.Stop()
	api.Stop()
	for i := range programs {
		programs[i].Quit()
	}
	secRunner.Quit()
	waitGroup.Wait()
	stopAll()
	config.SectionInterface.Deinitialize()
}
