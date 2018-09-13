package main

import (
	"os"
	"os/signal"
	"sync"

	c "git.amikhalev.com/amikhalev/grinklers/config"
	l "git.amikhalev.com/amikhalev/grinklers/logic"
	"git.amikhalev.com/amikhalev/grinklers/mqtt"
	log "github.com/Sirupsen/logrus"
	"github.com/joho/godotenv"
)

func main() {
	// channel which is notified on an interrupt signal
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	godotenv.Load()

	err := l.RpioSectionInit()
	if err != nil {
		log.WithError(err).Fatalf("error initializing sections")
	}

	config, err := c.LoadConfig()
	if err != nil {
		log.WithError(err).Fatalf("error loading config")
	}

	log.Info("writing back config")
	c.WriteConfig(&config)

	waitGroup := sync.WaitGroup{}

	secRunner := l.NewSectionRunner()
	secRunner.Start(&waitGroup)

	sections := config.Sections
	programs := config.Programs

	updater := mqtt.NewMQTTUpdater(&config, secRunner)

	log.Debug("initializing sections and programs")

	stopAll := func() {
		for i := range sections {
			sections[i].SetState(false)
		}
	}
	stopAll()

	for i := range programs {
		programs[i].Start(secRunner, &waitGroup)
	}

	log.WithFields(log.Fields{
		"lenSections": len(sections), "lenPrograms": len(programs),
	}).Info("initialized sections and programs")

	api := mqtt.NewMQTTApi(&config, secRunner)
	api.Start()

	updater.Start(api)

	<-sigc

	log.Info("cleaning up...")
	updater.Stop()
	api.Stop()
	for i := range programs {
		programs[i].Quit()
	}
	secRunner.Quit()
	waitGroup.Wait()
	stopAll()
	l.RpioSectionCleanup()
}
