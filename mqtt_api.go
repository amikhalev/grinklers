package grinklers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/eclipse/paho.mqtt.golang"
)

type responseData map[string]interface{}
type requestHandler func(message mqtt.Message, rData responseData) (err error)

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
		a.logger.WithFields(logrus.Fields{
			"username": username, "password": password,
		}).Debug("authenticating to mqtt server")
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
		a.UpdateAll()
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

// UpdateAll updates all mqtt data
func (a *MQTTApi) UpdateAll() (err error) {
	err = a.UpdatePrograms(a.config.Programs)
	if err != nil {
		return
	}
	err = a.UpdateSections(a.config.Sections)
	if err != nil {
		return
	}
	err = a.UpdateSectionRunner(&a.secRunner.State)
	return
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
	state.Lock()
	data, err := state.ToJSON(a.config.Sections)
	state.Unlock()
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

func (a *MQTTApi) subscribe() {
	reqPath := a.prefix + "/requests"
	resPath := a.prefix + "/responses"
	a.logger.WithField("path", reqPath).Debug("registering request handler")
	a.client.Subscribe(reqPath, 2, func(client mqtt.Client, message mqtt.Message) {
		var (
			data struct {
				Rid  int    `json:"rid"`
				Type string `json:"type"`
			}
			rData = make(responseData)
			err   error
		)

		defer func() {
			var (
				merr *MQTTError
				ok bool
			)
			if merr, ok = err.(*MQTTError); !ok {
				merr = NewInternalError(err)
			}
			if merr != nil {
				a.logger.WithError(merr).Info("error processing request")
				rData["result"] = "error"
				rData["code"] = merr.Code
				rData["message"] = merr.Message
				if merr.Name != "" {
					rData["name"] = merr.Name
				}
				if merr.Cause != nil {
					rData["cause"] = merr.Cause.Error()
					if e, ok := merr.Cause.(*json.SyntaxError); ok {
						rData["offset"] = e.Offset
					}
				}
			} else {
				rData["result"] = "success"
			}
			resBytes, err := json.Marshal(&rData)
			if err != nil {
				a.logger.WithError(err).Error("error marshaling response")
				return
			}
			client.Publish(resPath, 2, false, resBytes)
		}()

		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse api request: %v", err)
			return
		}

		rData["rid"] = data.Rid
		rData["type"] = data.Type

		var handler requestHandler
		switch data.Type {
		case "runProgram":
			handler = a.runProgram
		case "cancelProgram":
			handler = a.cancelProgram
		case "updateProgram":
			handler = a.updateProgram
		case "runSection":
			handler = a.runSection
		case "cancelSection":
			handler = a.cancelSection
		case "cancelSectionRunId":
			handler = a.cancelSectionRunID
		case "pauseSectionRunner":
			handler = a.pauseSectionRunner
		}

		if handler != nil {
			err = handler(message, rData)
		} else {
			err = NewMQTTError(EC_BadRequest, fmt.Sprintf("invalid api request type: %s", data.Type))
		}
	})
}

func parseDuration(durStr *string) (duration *time.Duration, err error) {
	if durStr == nil {
		err = NewNotSpecifiedError("duration")
		return
	}
	dur, err := time.ParseDuration(*durStr)
	duration = &dur
	if err != nil {
		err = NewParseError("duration", err)
	}
	return
}

func (a *MQTTApi) getProgram(progID *int) (program *Program, err error) {
	err = CheckRange(progID, "program ID", len(a.config.Programs))
	if err != nil {
		return
	}
	program = &a.config.Programs[*progID]
	return
}

func (a *MQTTApi) getSection(secID *int) (section Section, err error) {
	err = CheckRange(secID, "section ID", len(a.config.Sections))
	if err != nil {
		return
	}
	section = a.config.Sections[*secID]
	return
}

func (a *MQTTApi) runProgram(message mqtt.Message, rData responseData) (err error) {
	var data struct {
		ProgramID *int
	}
	err = json.Unmarshal(message.Payload(), &data)
	if err != nil {
		err = NewParseError("runProgram request", err)
		return
	}
	program, err := a.getProgram(data.ProgramID)
	if err != nil {
		return
	}
	program.Run()
	rData["Message"] = fmt.Sprintf("running program '%s'", program.Name)
	return
}

func (a *MQTTApi) cancelProgram(message mqtt.Message, rData responseData) (err error) {
	var data struct {
		ProgramID *int
	}
	err = json.Unmarshal(message.Payload(), &data)
	if err != nil {
		err = NewParseError("cancelProgram request", err)
		return
	}
	program, err := a.getProgram(data.ProgramID)
	if err != nil {
		return
	}
	program.Cancel()
	rData["Message"] = fmt.Sprintf("cancelled program '%s'", program.Name)
	return
}

func (a *MQTTApi) updateProgram(message mqtt.Message, rData responseData) (err error) {
	var data struct {
		ProgramID *int
		Data      ProgramJSON
	}
	err = json.Unmarshal(message.Payload(), &data)
	if err != nil {
		err = NewParseError("updateProgram request", err)
		return
	}
	program, err := a.getProgram(data.ProgramID)
	if err != nil {
		return
	}
	err = program.Update(data.Data, a.config.Sections)
	if err != nil {
		err = NewInvalidDataError("program update", err)
		return
	}
	programJSON, err := program.ToJSON(a.config.Sections)
	if err != nil {
		return
	}
	rData["Message"] = fmt.Sprintf("updated program '%s'", program.Name)
	rData["data"] = programJSON
	return
}

func (a *MQTTApi) runSection(message mqtt.Message, rData responseData) (err error) {
	var data struct {
		SectionID *int
		Duration  float64
	}
	err = json.Unmarshal(message.Payload(), &data)
	if err != nil {
		err = NewParseError("runSection request", err)
		return
	}
	sec, err := a.getSection(data.SectionID)
	if err != nil {
		return
	}
	duration := time.Duration(data.Duration * float64(time.Second))
	id := a.secRunner.QueueSectionRun(sec, duration)
	rData["Message"] = fmt.Sprintf("running section '%s' for %v", sec.Name(), duration)
	rData["runId"] = id
	return
}

func (a *MQTTApi) cancelSection(message mqtt.Message, rData responseData) (err error) {
	var data struct {
		SectionID *int
	}
	err = json.Unmarshal(message.Payload(), &data)
	if err != nil {
		err = NewParseError("cancelSection request", err)
		return
	}
	sec, err := a.getSection(data.SectionID)
	if err != nil {
		return
	}
	a.secRunner.CancelSection(sec)
	rData["Message"] = fmt.Sprintf("cancelled section '%s'", sec.Name())
	return
}

func (a *MQTTApi) cancelSectionRunID(message mqtt.Message, rData responseData) (err error) {
	var data struct {
		RunID *int32
	}
	err = json.Unmarshal(message.Payload(), &data)
	if err != nil || data.RunID == nil {
		err = NewParseError("cancelSectionRunId request", err)
		return
	}
	a.secRunner.CancelID(*data.RunID)
	rData["Message"] = fmt.Sprintf("cancelled section run with id %v", data.RunID)
	return
}

func (a *MQTTApi) pauseSectionRunner(message mqtt.Message, rData responseData) (err error) {
	var data struct {
		Paused *bool
	}
	err = json.Unmarshal(message.Payload(), &data)
	if err != nil || data.Paused == nil {
		err = NewParseError("parse pauseSectionRunner request", err)
		return
	}
	rData["paused"] = data.Paused
	if *data.Paused {
		a.secRunner.Pause()
		rData["Message"] = "paused section runner"
	} else {
		a.secRunner.Unpause()
		rData["Message"] = "unpaused section runner"
	}
	return
}
