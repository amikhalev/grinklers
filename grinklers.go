package main

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/inconshreveable/log15"
	"os"
	"os/signal"
	"encoding/json"
	"io/ioutil"
	"time"
	"net/url"
	"fmt"
	"strconv"
)

type ConfigData struct {
	Sections []Section
	Programs []Program
}

func createMqttOpts(broker string) (opts *mqtt.ClientOptions) {
	uri, err := url.Parse(broker)
	if err != nil {
		panic(err)
	}
	opts = mqtt.NewClientOptions()
	opts.AddBroker(uri.String())
	if uri.User != nil {
		username := uri.User.Username()
		opts.SetUsername(username)
		password, _ := uri.User.Password()
		opts.SetPassword(password)
		log.Debug("authenticating to mqtt server", "username", username, "password", password)
	}
	opts.SetClientID("grinklers-1")
	opts.SetWill("grinklers/connected", "false", 1, true)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Info("connected to mqtt broker")
	})
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Warn("lost connection to mqtt broker", "err", err)
	})
	return
}

func startMqtt() (client mqtt.Client) {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}
	opts := createMqttOpts(broker)
	client = mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Error("error connecting to mqtt broker", "err", token.Error())
	}
	return
}

var (
	configData ConfigData
	mqttClient mqtt.Client
	sectionRunner SectionRunner
)

func updateSections() {
	bytes, err := json.Marshal(configData.Sections)
	if err != nil {
		log.Error("error marshalling sections", "err", err)
	}
	token := mqttClient.Publish("grinklers/sections", 1, true, bytes)
	if token.Wait(); token.Error() != nil {
		log.Error("error publishing sections", "err", token.Error())
	}
	log.Debug("updating sections", "bytes", string(bytes))
}

func updatePrograms() {
	bytes, err := json.Marshal(configData.Programs)
	if err != nil {
		log.Error("error marshalling programs", "err", err)
	}
	token := mqttClient.Publish("grinklers/programs", 1, true, bytes)
	if token.Wait(); token.Error() != nil {
		log.Error("error publishing programs", "err", token.Error())
	}
	log.Debug("updating programs", "bytes", string(bytes))
}

func updater(onSectionUpdate chan *Section) {
	go func() {
		for {
			log.Debug("waiting for update")
			select {
			case <-onSectionUpdate:
				log.Debug("received update")
				Loop:
				for {
					select {
					case <-onSectionUpdate:
						log.Debug("received more update")
					default:
						break Loop
					}
				}
				updateSections()
			}
		}
	}()
	return
}

func checkRange(ref *int, name string, val int) (err error) {
	if ref == nil {
		err = fmt.Errorf("%s not specified", name)
		return
	}
	if *ref >= val {
		err = fmt.Errorf("%s out of range: %v", name, *ref)
		return
	}
	return
}

type ApiHandler func(client mqtt.Client, message mqtt.Message) (error)

func apiResponder(path string, handler ApiHandler) {
	mqttClient.Subscribe(path, 2, func(client mqtt.Client, message mqtt.Message) {
		var data struct {
			Rid int
		}
		var err error
		defer func() {
			if err != nil {
				log.Warn("error processing request", "err", err)
				resp := struct {
					Rid int `json:"rid,omitempty"`; Error string `json:"error"`
				}{data.Rid, err.Error()}
				res, _ := json.Marshal(&resp)
				client.Publish(path + "/response", 1, false, res)
			}
		}()
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse api request: %v", err)
			return
		}
		err = handler(client, message)
		return
	})
}

func mqttSubs() {
	apiResponder("grinklers/runProgram", func(client mqtt.Client, message mqtt.Message) (err error) {
		var data struct {
			ProgramId *int
		}
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse run program request: %v", err)
			return
		}
		programs := configData.Programs
		err = checkRange(data.ProgramId, "programId", len(programs))
		if err != nil {
			return
		}
		go programs[*data.ProgramId].Run()
		return
	})

	apiResponder("grinklers/runSection", func(client mqtt.Client, message mqtt.Message) (err error) {
		var data struct {
			SectionId *int
			Duration  *string
		}
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse run section request: %v", err)
			return
		}
		sections := configData.Sections
		err = checkRange(data.SectionId, "sectionId", len(sections))
		if err != nil {
			return
		}
		if data.Duration == nil {
			err = fmt.Errorf("no duration specified")
			return
		}
		duration, err := time.ParseDuration(*data.Duration)
		if err != nil {
			err = fmt.Errorf("could not parse section duration: %v", err)
			return
		}
		done := configData.Sections[*data.SectionId].RunForAsync(duration)
		go func() {
			<-done
		}()
		return
	})
}

func updateConnected(connected bool) (err error) {
	str := strconv.FormatBool(connected)
	token := mqttClient.Publish("grinklers/connected", 1, true, str)
	if token.Wait(); token.Error() != nil {
		return token.Error()
	}
	return
}

func main() {
	// channel which is notified on an interrupt signal
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	file, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Error("error reading config file", "err", err)
		os.Exit(1)
	}
	err = json.Unmarshal(file, &configData)
	if err != nil {
		log.Error("error parsing config file", "err", err)
		os.Exit(1)
	}

	sectionRunner = NewSectionRunner()

	onSectionUpdate := make(chan *Section, 10)

	log.Info("initing sections")
	for _, section := range configData.Sections {
		section.OnUpdate = onSectionUpdate
		section.Off()
	}

	mqttClient = startMqtt()
	defer mqttClient.Disconnect(250)

	updateConnected(true)

	mqttSubs()
	updater(onSectionUpdate)
	updatePrograms()

	<-sigc

	log.Info("cleaning up...")
	updateConnected(false)
	for _, section := range configData.Sections {
		section.Off()
	}
	Cleanup()
}