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

	err := g.InitSection()
	if err != nil {
		log.WithError(err).Fatalf("error initializing sections")
	}
	defer g.CleanupSection()

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

	updater := g.NewMQTTUpdater(&config)

	log.Debug("initializing sections and programs")
	for i := range sections {
		sections[i].SetState(false)
	}
	for i := range programs {
		programs[i].Start(secRunner, &waitGroup)
	}
	log.WithFields(log.Fields{
		"lenSections": len(sections), "lenPrograms": len(programs),
	}).Info("initialized sections and programs")

	api := g.NewMQTTApi(&config, secRunner)
	api.Start()
	defer api.Stop()

	updater.Start(api)
	updater.UpdateSections()
	updater.UpdatePrograms()
	defer updater.Stop()

	<-sigc

	log.Info("cleaning up...")
	for i := range programs {
		programs[i].Quit()
	}
	for i := range sections {
		sections[i].SetState(false)
	}
}
