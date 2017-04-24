package grinklers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// ConfigData is the app state after being read from config
type ConfigData struct {
	Sections []Section
	Programs []Program
}

func (c *ConfigData) ToJSON() (j ConfigDataJSON, err error) {
	j = ConfigDataJSON{}
	j.Sections = c.Sections
	j.Programs, err = ProgramsToJSON(c.Programs, c.Sections)
	return
}

type ConfigDataJSON struct {
	Sections RpioSections
	Programs ProgramsJSON
}

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
		dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
		configFile = dir + "/config.json"
	}
	return
}

var log = Logger.WithField("module", "config")
var configFile = findConfigFile()
var configMutex = &sync.Mutex{}

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
