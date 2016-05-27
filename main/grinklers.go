package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"github.com/joho/godotenv"
	g "github.com/amikhalev/grinklers"
	log "github.com/inconshreveable/log15"
)

type ConfigDataJson struct {
	Sections g.RpioSections
	Programs g.ProgramsJSON
}

func main() {
	// channel which is notified on an interrupt signal
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	godotenv.Load()

	var configData ConfigDataJson

	file, err := ioutil.ReadFile("./config.json")
	if err != nil {
		panic(fmt.Errorf("error reading config file: %v", err))
	}
	err = json.Unmarshal(file, &configData)
	if err != nil {
		panic(fmt.Errorf("error parsing config file: %v", err))
	}

	sections := configData.Sections
	log.Info("sections loaded", "sections", configData.Sections)
	programs, err := configData.Programs.ToPrograms(sections)
	if err != nil {
		panic(fmt.Errorf("invalid programs json: %v", err))
	}

	secRunner := g.NewSectionRunner()

	g.InitSection()
	defer g.CleanupSection()

	updater := g.NewMqttUpdater(sections, programs)

	log.Debug("initializing sections and programs")
	for i, _ := range sections {
		sections[i].SetState(false)
	}
	for i, _ := range programs {
		programs[i].Start(secRunner)
	}
	log.Info("initialized sections and programs")

	api := g.NewMqttApi(sections, programs, secRunner)
	api.Start()
	defer api.Stop()

	updater.Start(api)
	updater.UpdatePrograms()
	defer updater.Stop()

	<-sigc

	log.Info("cleaning up...")
	for i, _ := range sections {
		sections[i].SetState(false)
	}
}
