package main

import (
	"encoding/json"
	"fmt"
	"github.com/inconshreveable/log15"
	"io/ioutil"
	"os"
	"os/signal"
	"github.com/joho/godotenv"
)

var logger log15.Logger

func init() {
	logger = log15.New()
}

type ConfigData struct {
	Sections []RpioSection
	Programs []Program
}

var (
	configData    ConfigData
	sectionRunner SectionRunner
)

func initialize() {

}

func main() {
	// channel which is notified on an interrupt signal
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	godotenv.Load()

	file, err := ioutil.ReadFile("./config.json")
	if err != nil {
		panic(fmt.Errorf("error reading config file: %v", err))
	}
	err = json.Unmarshal(file, &configData)
	if err != nil {
		panic(fmt.Errorf("error parsing config file: %v", err))
	}

	sections, programs := configData.Sections, configData.Programs

	InitSection()
	sectionRunner = NewSectionRunner()

	onSectionUpdate, onProgramUpdate, stopUpdater := make(chan *RpioSection, 10), make(chan *Program, 10), make(chan int)

	logger.Debug("initializing sections and programs")
	for i, _ := range sections {
		section := &sections[i]
		section.OnUpdate = onSectionUpdate
		section.SetState(false)
	}
	for i, _ := range programs {
		program := &programs[i]
		program.OnUpdate = onProgramUpdate
		program.Start()
	}
	logger.Info("initialized sections and programs")

	mqttClient = startMqtt()
	defer mqttClient.Disconnect(250)

	updateConnected(true)

	mqttSubs()
	go updater(onSectionUpdate, onProgramUpdate, stopUpdater)
	updatePrograms()

	<-sigc

	logger.Info("cleaning up...")
	stopUpdater <- 0
	updateConnected(false)
	for i, _ := range sections {
		section := &sections[i]
		section.SetState(false)
	}
	CleanupSection()
}
