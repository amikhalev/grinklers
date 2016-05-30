package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"

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

	file, err := ioutil.ReadFile("./config.json")
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

	sections, programs, err := loadConfig()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	secRunner := g.NewSectionRunner()

	g.InitSection()
	defer g.CleanupSection()

	updater := g.NewMQTTUpdater(sections, programs)

	log.Debug("initializing sections and programs")
	for i, _ := range sections {
		sections[i].SetState(false)
	}
	for i, _ := range programs {
		programs[i].Start(secRunner)
	}
	log.WithFields(log.Fields{
		"lenSections": len(sections), "lenPrograms": len(programs),
	}).Info("initialized sections and programs")

	api := g.NewMQTTApi(sections, programs, secRunner)
	api.Start()
	defer api.Stop()

	updater.Start(api)
	updater.UpdatePrograms()
	defer updater.Stop()

	<-sigc

	log.Info("cleaning up...")
	for i, _ := range programs {
		programs[i].Quit()
	}
	for i, _ := range sections {
		sections[i].SetState(false)
	}
}
