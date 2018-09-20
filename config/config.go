package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"git.amikhalev.com/amikhalev/grinklers/datamodel"
	"git.amikhalev.com/amikhalev/grinklers/logic"
	"git.amikhalev.com/amikhalev/grinklers/util"
	rpio "github.com/stianeikeland/go-rpio"
)

// ConfigData is the app state after being read from config
type ConfigData struct {
	Pins             []uint16
	SectionInterface logic.SectionInterface
	Sections         []logic.Section
	Programs         []*logic.Program
}

// ToJSON converts a ConfigData to a ConfigDataJSON
func (c *ConfigData) ToJSON() (j ConfigDataJSON) {
	j = ConfigDataJSON{}
	j.SectionInterface = SectionInterfaceJSON{Pins: c.Pins}
	j.Sections = c.Sections
	j.Programs = datamodel.ProgramsToJSON(c.Programs)
	return
}

type SectionInterfaceJSON struct {
	Pins []uint16 `json:"pins"`
}

func (ij *SectionInterfaceJSON) ToInterface() logic.SectionInterface {
	rpi := os.Getenv("RPI") == "true" // TODO: base this off go-config
	if rpi {
		pins := make(logic.RpioPins, len(ij.Pins))
		for i, pin := range ij.Pins {
			pins[i] = (rpio.Pin)(pin)
		}
		return logic.NewRpioSectionInterface(pins)
	} else {
		return logic.NewMockSectionInterface(len(ij.Pins))
	}
}

// ConfigDataJSON is the JSON form of config data
type ConfigDataJSON struct {
	SectionInterface SectionInterfaceJSON
	Sections         logic.Sections         `json:"sections"`
	Programs         datamodel.ProgramsJSON `json:"programs"`
}

// ToConfigData converts a ConfigDataJSON to a ConfigData
func (j *ConfigDataJSON) ToConfigData() (c ConfigData, err error) {
	c = ConfigData{}
	c.Pins = j.SectionInterface.Pins
	c.SectionInterface = j.SectionInterface.ToInterface()
	c.Sections = j.Sections
	c.Programs, err = j.Programs.ToPrograms(c.Sections)
	if err != nil {
		err = fmt.Errorf("invalid programs json: %v", err)
	}
	return
}

func findConfigFile() (configFile string) {
	configFile = os.Getenv("CONFIG")
	if configFile == "" {
		dir, _ := os.Getwd()
		configFile = dir + "/config.json"
	}
	return
}

var log = util.Logger.WithField("module", "config")
var configFile = findConfigFile()
var configMutex = &sync.Mutex{}

// LoadConfig loads a ConfigData from the config file
func LoadConfig() (config ConfigData, err error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	var j ConfigDataJSON

	log.Debugf("loading config from %v", configFile)
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		err = fmt.Errorf("could not read config file: %v", err)
		return
	}
	err = json.Unmarshal(file, &j)
	if err != nil {
		err = fmt.Errorf("could not parse config file: %v", err)
		return
	}

	config, err = j.ToConfigData()
	return
}

// WriteConfig writes a ConfigData to the config file
func WriteConfig(configData *ConfigData) (err error) {
	configMutex.Lock()
	defer configMutex.Unlock()

	log.Debugf("writing config to %v", configFile)
	data := configData.ToJSON()
	if err != nil {
		err = fmt.Errorf("invalid config data: %v", err)
	}

	bytes, err := json.MarshalIndent(&data, "", "  ")
	if err != nil {
		err = fmt.Errorf("could not parse config file: %v", err)
		return
	}

	err = ioutil.WriteFile(configFile, bytes, 0)
	if err != nil {
		err = fmt.Errorf("could not read config file: %v", err)
		return
	}
	return
}
