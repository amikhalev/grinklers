package grinklers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

// ConfigData is the app state after being read from config
type ConfigData struct {
	Sections []Section
	Programs []Program
}

// ToJSON converts a ConfigData to a ConfigDataJSON
func (c *ConfigData) ToJSON() (j ConfigDataJSON, err error) {
	j = ConfigDataJSON{}
	j.Sections = c.Sections
	j.Programs, err = ProgramsToJSON(c.Programs, c.Sections)
	return
}

// ConfigDataJSON is the JSON form of config data
type ConfigDataJSON struct {
	Sections RpioSections `json:"sections"`
	Programs ProgramsJSON `json:"programs"`
}

// ToConfigData converts a ConfigDataJSON to a ConfigData
func (j *ConfigDataJSON) ToConfigData() (c ConfigData, err error) {
	c = ConfigData{}
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

var log = Logger.WithField("module", "config")
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
	data, err := configData.ToJSON()
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
