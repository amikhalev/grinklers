package main

import (
	"os"
	"os/signal"
	"sync"

	log "github.com/Sirupsen/logrus"
	g "github.com/amikhalev/grinklers"
	"github.com/joho/godotenv"
)

func main() {
	// channel which is notified on an interrupt signal
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	godotenv.Load()

	err := g.RpioSectionInit()
	if err != nil {
		log.WithError(err).Fatalf("error initializing sections")
	}

	config, err := g.LoadConfig()
	if err != nil {
		log.WithError(err).Fatalf("error loading config")
	}

	log.Info("writing back config")
	g.WriteConfig(&config)

	waitGroup := sync.WaitGroup{}

	secRunner := g.NewSectionRunner()
	secRunner.Start(&waitGroup)

	sections := config.Sections
	programs := config.Programs

	updater := g.NewMQTTUpdater(&config, secRunner)

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

	api := g.NewMQTTApi(&config, secRunner)
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
	g.RpioSectionCleanup()
}
