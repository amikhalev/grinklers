package main

import (
	"os"
	"os/signal"
	"sync"

	"git.amikhalev.com/amikhalev/grinklers/http"

	c "git.amikhalev.com/amikhalev/grinklers/config"
	l "git.amikhalev.com/amikhalev/grinklers/logic"
	"git.amikhalev.com/amikhalev/grinklers/mqtt"
	"git.amikhalev.com/amikhalev/grinklers/util"
	log "github.com/Sirupsen/logrus"
	"github.com/joho/godotenv"
)

var logger = util.Logger.WithField("module", "server")

func main() {
	util.InitLogLevel()
	// channel which is notified on an interrupt signal
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	godotenv.Load()

	config, err := c.LoadConfig()
	if err != nil {
		logger.WithError(err).Fatalf("error loading config")
	}

	err = config.SectionInterface.Initialize()
	if err != nil {
		logger.WithError(err).Fatalf("error initializing sections")
	}

	waitGroup := sync.WaitGroup{}

	secRunner := l.NewSectionRunner(config.SectionInterface)
	secRunner.Start(&waitGroup)

	sections := config.Sections
	programs := config.Programs

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

	// httpConfig := &http.Config{ApiURL: apiUrl, DeviceRegistrationToken: deviceRegToken}
	httpConfig := config.HTTPConfig
	httpApiClient := http.NewAPIClient(httpConfig)
	if config.DeviceData == nil {
		err = httpApiClient.Register()
		if err != nil {
			logger.WithError(err).Error("error registering device")
		} else {
			config.DeviceData = httpApiClient.Device
		}
	}

	logger.Info("writing back config")
	c.WriteConfig(&config)

	httpApiClient.Device = config.DeviceData
	connectData, err := httpApiClient.Connect()
	if err != nil {
		logger.WithError(err).Error("error connecting device")
	}

	api := mqtt.NewMQTTApi(&config, secRunner)
	api.Start(connectData)

	updater := mqtt.NewMQTTUpdater(&config, secRunner)

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
