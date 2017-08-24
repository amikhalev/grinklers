package grinklers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/eclipse/paho.mqtt.golang"
)

type respData map[string]interface{}

type apiHandlerFunc func(client mqtt.Client, message mqtt.Message, rData respData) (err error)

// MQTTApi encapsulates all functionality exposed over MQTT
type MQTTApi struct {
	config    *ConfigData
	secRunner *SectionRunner
	client    mqtt.Client
	prefix    string
	logger    *logrus.Entry
}

// NewMQTTApi creates a new MQTTApi that uses the specified data
func NewMQTTApi(config *ConfigData, secRunner *SectionRunner) *MQTTApi {
	return &MQTTApi{
		config, secRunner,
		nil, "",
		Logger.WithField("module", "MQTTApi"),
	}
}

func (a *MQTTApi) createMQTTOpts(brokerURI *url.URL, cid string) (opts *mqtt.ClientOptions) {
	opts = mqtt.NewClientOptions()
	opts.AddBroker(brokerURI.String())
	if brokerURI.User != nil {
		username := brokerURI.User.Username()
		opts.SetUsername(username)
		password, _ := brokerURI.User.Password()
		opts.SetPassword(password)
		a.logger.WithFields(logrus.Fields{"username": username, "password": password}).Debug("authenticating to mqtt server")
	}
	opts.SetClientID(cid)
	return
}

// Start connects to the MQTT broker and listens to the API topics
func (a *MQTTApi) Start() (err error) {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}
	brokerURI, err := url.Parse(broker)
	if err != nil {
		err = fmt.Errorf("error parsing MQTT_BROKER: %v", err)
		return
	}
	cid := os.Getenv("MQTT_CID")
	if cid == "" {
		cid = "grinklers-1"
	}
	if brokerURI.Path != "" {
		a.prefix = brokerURI.Path
	} else {
		a.prefix = "grinklers"
	}
	a.logger.Debugf("broker prefix: '%s'", a.prefix)

	opts := a.createMQTTOpts(brokerURI, cid)
	opts.SetWill(a.prefix+"/connected", "false", 1, true)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		a.logger.Info("connected to mqtt broker")
		a.updateConnected(true)
	})
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		a.logger.WithError(err).Warn("lost connection to mqtt broker")
	})
	a.client = mqtt.NewClient(opts)

	if token := a.client.Connect(); token.Wait() && token.Error() != nil {
		a.logger.WithError(token.Error()).Error("error connecting to mqtt broker")
	}

	a.subscribe()

	return
}

// Stop disconnects from the broker
func (a *MQTTApi) Stop() {
	if a.client.IsConnected() {
		a.logger.Info("disconnecting from mqtt broker")
		a.updateConnected(false)
		a.client.Disconnect(250)
	} else {
		a.logger.Warn("was never connected to broker")
	}
}

// Client gets the MQTT client used by the MQTTApi
func (a *MQTTApi) Client() mqtt.Client {
	return a.client
}

// Prefix gets the topic prefix of this MQTTApi
func (a *MQTTApi) Prefix() string {
	return a.prefix
}

func (a *MQTTApi) updateConnected(connected bool) (err error) {
	str := strconv.FormatBool(connected)
	token := a.client.Publish(a.prefix+"/connected", 1, true, str)
	if token.Wait(); token.Error() != nil {
		return token.Error()
	}
	return
}

func (a *MQTTApi) subscribeHandler(path string, handler apiHandlerFunc) {
	p := a.prefix + path
	a.logger.WithField("path", p).Debug("registering handler")
	a.client.Subscribe(p, 2, func(client mqtt.Client, message mqtt.Message) {
		var (
			data struct {
				Rid int `json:"rid"`
			}
			rData = make(respData)
			err   error
		)

		rData["reqTopic"] = message.Topic()

		defer func() {
			if pan := recover(); pan != nil {
				a.logger.WithField("panic", pan).Warn("panic in api responder")
				err = fmt.Errorf("internal server panic: %v", pan)
			}
			topic := fmt.Sprintf("%s/responses/%d", a.prefix, data.Rid)
			if err != nil {
				a.logger.WithError(err).Info("error processing request")
				rData["error"] = err.Error()
				if e, ok := err.(*json.SyntaxError); ok {
					rData["offset"] = e.Offset
				}
			}
			resBytes, _ := json.Marshal(&rData)
			client.Publish(topic, 1, false, resBytes)
		}()

		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse api request: %v", err)
			return
		}

		err = handler(client, message, rData)
	})
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

func (a *MQTTApi) parseProgramPath(path string) (program *Program, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 4 {
		err = fmt.Errorf("invalid path: %s", path)
		return
	}
	progStr := parts[2]
	progID, err := strconv.Atoi(progStr)
	if err != nil {
		return
	}
	err = CheckRange(&progID, a.prefix+"/programs/id", len(a.config.Programs))
	if err != nil {
		return
	}
	program = &a.config.Programs[progID]
	return
}

func (a *MQTTApi) parseSectionPath(path string) (section Section, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 4 {
		err = fmt.Errorf("invalid path: %s", path)
		return
	}
	secStr := parts[2]
	secID, err := strconv.Atoi(secStr)
	if err != nil {
		return
	}
	err = CheckRange(&secID, a.prefix+"/sections/id", len(a.config.Sections))
	if err != nil {
		return
	}
	section = a.config.Sections[secID]
	return
}

func (a *MQTTApi) subscribe() {
	a.subscribeHandler("/programs/+/run", func(client mqtt.Client, message mqtt.Message, rData respData) (err error) {
		program, err := a.parseProgramPath(message.Topic())
		if err != nil {
			return
		}
		program.Run()
		rData["message"] = fmt.Sprintf("running program '%s'", program.Name)
		return
	})

	a.subscribeHandler("/programs/+/cancel", func(client mqtt.Client, message mqtt.Message, rData respData) (err error) {
		program, err := a.parseProgramPath(message.Topic())
		if err != nil {
			return
		}
		program.Cancel()
		rData["message"] = fmt.Sprintf("cancelled program '%s'", program.Name)
		return
	})

	a.subscribeHandler("/programs/+/update", func(client mqtt.Client, message mqtt.Message, rData respData) (err error) {
		program, err := a.parseProgramPath(message.Topic())
		if err != nil {
			return
		}
		var data ProgramJSON
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse program update: %v", err)
			return
		}
		err = program.Update(data, a.config.Sections)
		if err != nil {
			err = fmt.Errorf("could not process program update: %v", err)
		}
		programJSON, err := program.ToJSON(a.config.Sections)
		if err != nil {
			return
		}
		rData["message"] = fmt.Sprintf("updated program '%s'", program.Name)
		rData["data"] = programJSON
		return
	})

	a.subscribeHandler("/sections/+/run", func(client mqtt.Client, message mqtt.Message, rData respData) (err error) {
		sec, err := a.parseSectionPath(message.Topic())
		if err != nil {
			return
		}
		var data struct {
			Duration float64
		}
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse run section request: %v", err)
			return
		}
		duration := time.Duration(data.Duration * float64(time.Second))
		id := a.secRunner.QueueSectionRun(sec, duration)
		rData["message"] = fmt.Sprintf("running section '%s' for %v", sec.Name(), duration)
		rData["new_id"] = id
		return
	})

	a.subscribeHandler("/sections/+/cancel", func(client mqtt.Client, message mqtt.Message, rData respData) (err error) {
		sec, err := a.parseSectionPath(message.Topic())
		if err != nil {
			return
		}
		a.secRunner.CancelSection(sec)
		rData["message"] = fmt.Sprintf("cancelled section '%s'", sec.Name())
		return
	})

	a.subscribeHandler("/section_runner/cancel_id", func(client mqtt.Client, message mqtt.Message, rData respData) (err error) {
		var data struct {
			ID int32
		}
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse section_runner/cancel_id request: %v", err)
			return
		}
		a.secRunner.CancelID(data.ID)
		rData["message"] = fmt.Sprintf("cancelled section run with id %v", data.ID)
		return
	})
}

// MQTTUpdater updates MQTT topics with the current state of the application
type MQTTUpdater struct {
	config                *ConfigData
	onSectionUpdate       chan SecUpdate
	onProgramUpdate       chan ProgUpdate
	onSectionRunnerUpdate chan *SRState
	stop                  chan int
	api                   *MQTTApi
	logger                *logrus.Entry
}

// UpdateSectionData updates the topic for the specified section
func (a *MQTTApi) UpdateSectionData(index int, sec Section) (err error) {
	bytes, err := json.Marshal(sec)
	if err != nil {
		err = fmt.Errorf("error marshalling section: %v", err)
		return
	}
	a.client.Publish(fmt.Sprintf("%s/sections/%d", a.prefix, index), 1, true, bytes)
	return
}

// UpdateSectionState updates the topic for the current state of the section
func (a *MQTTApi) UpdateSectionState(index int, sec Section) (err error) {
	bytes := []byte(strconv.FormatBool(sec.State()))
	a.client.Publish(fmt.Sprintf("%s/sections/%d/state", a.prefix, index), 1, true, bytes)
	return
}

// UpdateSections updates the topics for all the specified sections
func (a *MQTTApi) UpdateSections(sections []Section) (err error) {
	lenSections := len(sections)
	bytes := []byte(strconv.Itoa(lenSections))
	a.client.Publish(a.prefix+"/sections", 1, true, bytes)
	for i, sec := range sections {
		err = a.UpdateSectionData(i, sec)
		if err != nil {
			return
		}
		err = a.UpdateSectionState(i, sec)
		if err != nil {
			return
		}
	}
	//logger.Debug("updated sections", "bytes", string(bytes))
	return
}

// UpdateProgramData updates the topic for the data about the specified Program
func (a *MQTTApi) UpdateProgramData(index int, prog *Program) (err error) {
	data, err := prog.ToJSON(a.config.Sections)
	if err != nil {
		err = fmt.Errorf("error converting programs to json: %v", err)
		return
	}
	bytes, err := json.Marshal(&data)
	if err != nil {
		err = fmt.Errorf("error marshalling program: %v", err)
		return
	}
	a.client.Publish(fmt.Sprintf("%s/programs/%d", a.prefix, index), 1, true, bytes)
	return
}

// UpdateProgramRunning updates the topic for the current running state of the Program
func (a *MQTTApi) UpdateProgramRunning(index int, prog *Program) (err error) {
	bytes := []byte(strconv.FormatBool(prog.Running()))
	a.client.Publish(fmt.Sprintf("%s/programs/%d/running", a.prefix, index), 1, true, bytes)
	return
}

// UpdatePrograms updates the topics for all the specified Programs
func (a *MQTTApi) UpdatePrograms(programs []Program) (err error) {
	lenPrograms := len(programs)
	bytes := []byte(strconv.Itoa(lenPrograms))
	a.client.Publish(a.prefix+"/programs", 1, true, bytes)
	for i := range programs {
		prog := &programs[i]
		err = a.UpdateProgramData(i, prog)
		if err != nil {
			return
		}
		err = a.UpdateProgramRunning(i, prog)
		if err != nil {
			return
		}
	}
	//logger.Debug("updated programs", "bytes", string(bytes))
	return
}

// UpdateSectionRunner updates the current section_runner state with the specified SRState
func (a *MQTTApi) UpdateSectionRunner(state *SRState) (err error) {
	state.Mu.Lock()
	data, err := state.ToJSON(a.config.Sections)
	state.Mu.Unlock()
	if err != nil {
		return
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	a.client.Publish(fmt.Sprintf("%s/section_runner", a.prefix), 1, true, bytes)
	return
}

// NewMQTTUpdater creates a new MQTTUpdater which uses the specified state
func NewMQTTUpdater(config *ConfigData, sectionRunner *SectionRunner) *MQTTUpdater {
	onSectionUpdate := make(chan SecUpdate, 10)
	onProgramUpdate := make(chan ProgUpdate, 10)
	onSectionRunnerUpdate := make(chan *SRState, 10)
	stop := make(chan int)
	for i := range config.Sections {
		config.Sections[i].SetOnUpdate(onSectionUpdate)
	}
	for i := range config.Programs {
		config.Programs[i].OnUpdate = onProgramUpdate
	}
	sectionRunner.OnUpdateState = onSectionRunnerUpdate
	return &MQTTUpdater{
		config,
		onSectionUpdate, onProgramUpdate, onSectionRunnerUpdate, stop, nil,
		Logger.WithField("module", "MQTTUpdater"),
	}
}

// UpdateSections updates the topics for all sections
func (u *MQTTUpdater) UpdateSections() {
	u.api.UpdateSections(u.config.Sections)
}

// UpdatePrograms updates topics for all programs
func (u *MQTTUpdater) UpdatePrograms() {
	u.api.UpdatePrograms(u.config.Programs)
}

func (u *MQTTUpdater) run() {
	u.logger.Debug("starting updater")
	for {
		//logger.Debug("waiting for update")
		select {
		case <-u.stop:
			return
		case secUpdate := <-u.onSectionUpdate:
			//logger.Debug("sec update")
			ExhaustChan(u.onSectionUpdate)

			index := -1
			for i, s := range u.config.Sections {
				if s == secUpdate.Sec {
					index = i
				}
			}
			if index == -1 {
				u.logger.Panicf("invalid section update recieved: %v", secUpdate.Sec)
			}

			var err error
			switch secUpdate.Type {
			case SecUpdateData:
				err = u.api.UpdateSectionData(index, secUpdate.Sec)
				if err == nil {
					err = WriteConfig(u.config)
				}
			case SecUpdateState:
				err = u.api.UpdateSectionState(index, secUpdate.Sec)
			default:
			}
			if err != nil {
				u.logger.WithError(err).Error("error updating sections")
			}
		case progUpdate := <-u.onProgramUpdate:
			//logger.Debug("prog update")
			ExhaustChan(u.onProgramUpdate)

			index := -1
			for i := range u.config.Programs {
				if &u.config.Programs[i] == progUpdate.Prog {
					index = i
				}
			}
			if index == -1 {
				u.logger.Panicf("invalid program update recieved: %v", progUpdate.Prog)
			}

			var err error
			switch progUpdate.Type {
			case pupdateData:
				err = u.api.UpdateProgramData(index, progUpdate.Prog)
				if err == nil {
					err = WriteConfig(u.config)
				}
			case pupdateRunning:
				err = u.api.UpdateProgramRunning(index, progUpdate.Prog)
			default:
			}
			if err != nil {
				u.logger.WithError(err).Error("error updating sections")
			}
		case srState := <-u.onSectionRunnerUpdate:
			ExhaustChan(u.onSectionRunnerUpdate)
			srState.Mu.Lock()
			u.logger.WithField("srState", *srState).Debugf("section runner update")
			srState.Mu.Unlock()

			err := u.api.UpdateSectionRunner(srState)
			if err != nil {
				u.logger.WithError(err).Error("error updating section runner state")
			}
		}
	}
}

// Start starts the MQTTUpdater to listen and update topics
func (u *MQTTUpdater) Start(api *MQTTApi) {
	u.api = api
	go u.run()
}

// Stop stops the updater from updating topics
func (u *MQTTUpdater) Stop() {
	u.stop <- 0
}
