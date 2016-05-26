package main

import (
	"encoding/json"
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"net/url"
	"os"
	"strconv"
	"time"
)

var (
	mqttClient mqtt.Client
	mqttBasePath string = "grinklers"
)

func createMqttOpts(broker string, cid string) (opts *mqtt.ClientOptions) {
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
		logger.Debug("authenticating to mqtt server", "username", username, "password", password)
	}
	opts.SetClientID(cid)
	opts.SetWill("grinklers/connected", "false", 1, true)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		logger.Info("connected to mqtt broker")
	})
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		logger.Warn("lost connection to mqtt broker", "err", err)
	})
	return
}

func startMqtt() (client mqtt.Client) {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}
	cid := os.Getenv("MQTT_CID")
	if cid == "" {
		cid = "grinklers-1"
	}
	opts := createMqttOpts(broker, cid)
	client = mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Error("error connecting to mqtt broker", "err", token.Error())
	}
	return
}

func updateSections() {
	bytes, err := json.Marshal(configData.Sections)
	if err != nil {
		logger.Error("error marshalling sections", "err", err)
	}
	token := mqttClient.Publish(mqttBasePath + "/sections", 1, true, bytes)
	if token.Wait(); token.Error() != nil {
		logger.Error("error publishing sections", "err", token.Error())
	}
	//logger.Debug("updating sections", "bytes", string(bytes))
}

func updatePrograms() {
	bytes, err := json.Marshal(configData.Programs)
	if err != nil {
		logger.Error("error marshalling programs", "err", err)
	}
	token := mqttClient.Publish(mqttBasePath + "/programs", 1, true, bytes)
	if token.Wait(); token.Error() != nil {
		logger.Error("error publishing programs", "err", token.Error())
	}
	//logger.Debug("updating programs", "bytes", string(bytes))
}

func updateConnected(connected bool) (err error) {
	str := strconv.FormatBool(connected)
	token := mqttClient.Publish(mqttBasePath + "/connected", 1, true, str)
	if token.Wait(); token.Error() != nil {
		return token.Error()
	}
	return
}

func updater(onSectionUpdate <-chan *RpioSection, onProgramUpdate <-chan *Program, stop <-chan int) {
	for {
		//logger.Debug("waiting for update")
		select {
		case <-stop:
			return
		case <-onSectionUpdate:
			//logger.Debug("sec update")
			ExhaustChan(onSectionUpdate)
			updateSections()
		case <-onProgramUpdate:
			//logger.Debug("prog update")
			ExhaustChan(onProgramUpdate)
			updatePrograms()
		}
	}
}

type ApiHandler func(client mqtt.Client, message mqtt.Message) (resp interface{}, err error)

type respJson struct {
	Rid      int    `json:"rid,omitempty"`
	Response interface{} `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

func apiResponder(path string, handler ApiHandler) {
	mqttClient.Subscribe(path, 2, func(client mqtt.Client, message mqtt.Message) {
		var data struct {
			Rid int
		}
		var (
			res interface{}; err error
		)

		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse api request: %v", err)
			return
		}

		defer func () {
			if pan := recover(); pan != nil {
				logger.Warn("panic in api responder", "panic", pan)
				err = fmt.Errorf("internal server panic: %v", pan)
			}
			var rData respJson
			if err != nil {
				logger.Info("error processing request", "err", err)
				rData = respJson{data.Rid, nil, err.Error()}
			} else {
				rData = respJson{data.Rid, res, ""}
			}
			resBytes, _ := json.Marshal(&rData)
			client.Publish(path + "/response", 1, false, resBytes)
		}()

		res, err = handler(client, message)
	})
}

type programReqJson struct {
	ProgramId *int
}

func parseProgramJson(payload []byte) (data programReqJson, err error) {
	err = json.Unmarshal(payload, &data)
	if err != nil {
		err = fmt.Errorf("could not parse program request: %v", err)
	}
	return
}

func parseDuration(durStr *string) (duration *time.Duration, err error) {
	if durStr == nil {
		err = fmt.Errorf("no duration specified")
		return
	}
	dur, err := time.ParseDuration(*durStr)
	duration = &dur
	if err != nil {
		err = fmt.Errorf("could not parse section duration: %v", err)
	}
	return
}

func mqttSubs() {
	apiResponder(mqttBasePath + "/runProgram", func(client mqtt.Client, message mqtt.Message) (res interface{}, err error) {
		data, err := parseProgramJson(message.Payload())
		if err != nil {
			return
		}
		programs := configData.Programs
		err = CheckRange(data.ProgramId, "programId", len(programs))
		if err != nil {
			return
		}
		program := &programs[*data.ProgramId]
		program.Run()
		res = fmt.Sprintf("running program '%s'", program.Name)
		return
	})

	apiResponder(mqttBasePath + "/cancelProgram", func(client mqtt.Client, message mqtt.Message) (res interface{}, err error) {
		data, err := parseProgramJson(message.Payload())
		if err != nil {
			return
		}
		programs := configData.Programs
		err = CheckRange(data.ProgramId, "programId", len(programs))
		if err != nil {
			return
		}
		program := &programs[*data.ProgramId]
		program.Cancel()
		res = fmt.Sprintf("cancelled program '%s'", program.Name)
		return
	})

	apiResponder(mqttBasePath + "/runSection", func(client mqtt.Client, message mqtt.Message) (res interface{}, err error) {
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
		err = CheckRange(data.SectionId, "sectionId", len(sections))
		if err != nil {
			return
		}
		sec := &configData.Sections[*data.SectionId]
		duration, err := parseDuration(data.Duration)
		if err != nil {
			return
		}
		done := sec.RunForAsync(*duration)
		go func() {
			<-done
		}()
		res = fmt.Sprintf("running section '%s' for %v", sec.Name(), duration)
		return
	})

	apiResponder(mqttBasePath + "/cancelSection", func(client mqtt.Client, message mqtt.Message) (res interface{}, err error) {
		var data struct {
			SectionId *int
		}
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse cancel section request: %v", err)
			return
		}
		sections := configData.Sections
		err = CheckRange(data.SectionId, "sectionId", len(sections))
		if err != nil {
			return
		}
		sec := &configData.Sections[*data.SectionId]
		sec.Cancel()
		res = fmt.Sprintf("cancelled section '%s'", sec.Name())
		return
	})
}
