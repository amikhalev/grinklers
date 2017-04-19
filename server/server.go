package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"

	"sync"

	"path/filepath"

	log "github.com/Sirupsen/logrus"
	g "github.com/amikhalev/grinklers"
	"github.com/joho/godotenv"
)

type ConfigDataJson struct {
	Sections g.RpioSections
	Programs g.ProgramsJSON
}

func loadConfig() (sections []g.Section, programs []g.Program, err error) {
	var configData ConfigDataJson

	configFile := os.Getenv("CONFIG")
	if configFile == "" {
		dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
		configFile = dir + "/config.json"
	}

	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		err = fmt.Errorf("could not read config file: %v", err)
		return
	}
	err = json.Unmarshal(file, &configData)
	if err != nil {
		err = fmt.Errorf("could not parse config file: %v", err)
		return
	}

	sections = configData.Sections
	programs, err = configData.Programs.ToPrograms(sections)
	if err != nil {
		err = fmt.Errorf("invalid programs json: %v", err)
	}
	return
}

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

	sections, programs, err := loadConfig()
	if err != nil {
		log.WithError(err).Fatalf("error loading config")
	}

	waitGroup := sync.WaitGroup{}

	secRunner := g.NewSectionRunner()
	secRunner.Start(&waitGroup)

	updater := g.NewMQTTUpdater(sections, programs)

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

	api := g.NewMQTTApi(sections, programs, secRunner)
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
