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
	sections  []Section
	programs  []Program
	secRunner *SectionRunner
	client    mqtt.Client
	prefix    string
	logger    *logrus.Entry
}

// NewMQTTApi creates a new MQTTApi that uses the specified data
func NewMQTTApi(sections []Section, programs []Program, secRunner *SectionRunner) *MQTTApi {
	return &MQTTApi{
		sections, programs, secRunner,
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
	a.logger.Info("disconnecting from mqtt broker")
	a.updateConnected(false)
	a.client.Disconnect(250)
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

func (a *MQTTApi) subscribeHandler(p string, handler apiHandlerFunc) {
	path := a.prefix + p
	a.logger.WithField("path", path).Debug("registering handler")
	a.client.Subscribe(path, 2, func(client mqtt.Client, message mqtt.Message) {
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
	err = CheckRange(&progID, a.prefix+"/programs/id", len(a.programs))
	if err != nil {
		return
	}
	program = &a.programs[progID]
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
	err = CheckRange(&secID, a.prefix+"/sections/id", len(a.sections))
	if err != nil {
		return
	}
	section = a.sections[secID]
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

	a.subscribeHandler("/sections/+/run", func(client mqtt.Client, message mqtt.Message, rData respData) (err error) {
		sec, err := a.parseSectionPath(message.Topic())
		if err != nil {
			return
		}
		var data struct {
			Duration *string
		}
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse run section request: %v", err)
			return
		}
		duration, err := parseDuration(data.Duration)
		if err != nil {
			return
		}
		done := a.secRunner.RunSectionAsync(sec, *duration)
		go func() {
			<-done
		}()
		rData["message"] = fmt.Sprintf("running section '%s' for %v", sec.Name(), duration)
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
}

// MQTTUpdater updates MQTT topics with the current state of the application
type MQTTUpdater struct {
	sections        []Section
	programs        []Program
	onSectionUpdate chan SecUpdate
	onProgramUpdate chan ProgUpdate
	stop            chan int
	api             *MQTTApi
	logger          *logrus.Entry
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
	data, err := prog.ToJSON(a.sections)
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

// NewMQTTUpdater creates a new MQTTUpdater which uses the specified state
func NewMQTTUpdater(sections []Section, programs []Program) *MQTTUpdater {
	onSectionUpdate, onProgramUpdate, stop := make(chan SecUpdate, 10), make(chan ProgUpdate, 10), make(chan int)
	for i := range sections {
		sections[i].SetOnUpdate(onSectionUpdate)
	}
	for i := range programs {
		programs[i].OnUpdate = onProgramUpdate
	}
	return &MQTTUpdater{
		sections, programs,
		onSectionUpdate, onProgramUpdate, stop, nil,
		Logger.WithField("module", "MQTTUpdater"),
	}
}

// UpdateSections updates the topics for all sections
func (u *MQTTUpdater) UpdateSections() {
	u.api.UpdateSections(u.sections)
}

// UpdatePrograms updates topics for all programs
func (u *MQTTUpdater) UpdatePrograms() {
	u.api.UpdatePrograms(u.programs)
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
			for i, s := range u.sections {
				if s == secUpdate.Sec {
					index = i
				}
			}
			if index == -1 {
				u.logger.Panicf("invalid section update recieved: %v", secUpdate.Sec)
			}

			var err error
			switch secUpdate.Type {
			case supdateData:
				err = u.api.UpdateSectionData(index, secUpdate.Sec)
			case supdateState:
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
			for i := range u.programs {
				if &u.programs[i] == progUpdate.Prog {
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
			case pupdateRunning:
				err = u.api.UpdateProgramRunning(index, progUpdate.Prog)
			default:
			}
			if err != nil {
				u.logger.WithError(err).Error("error updating sections")
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
