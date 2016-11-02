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

type MQTTApi struct {
	sections  []Section
	programs  []Program
	secRunner *SectionRunner
	client    mqtt.Client
	prefix    string
	logger    *logrus.Entry
}

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

func (a *MQTTApi) Stop() {
	a.logger.Info("disconnecting from mqtt broker")
	a.updateConnected(false)
	a.client.Disconnect(250)
}

func (a *MQTTApi) Client() mqtt.Client {
	return a.client
}

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
			rData respData = make(respData)
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
	progId, err := strconv.Atoi(progStr)
	if err != nil {
		return
	}
	err = CheckRange(&progId, a.prefix+"/programs/id", len(a.programs))
	if err != nil {
		return
	}
	program = &a.programs[progId]
	return
}

func (a *MQTTApi) parseSectionPath(path string) (section Section, err error) {
	parts := strings.Split(path, "/")
	if len(parts) != 4 {
		err = fmt.Errorf("invalid path: %s", path)
		return
	}
	secStr := parts[2]
	secId, err := strconv.Atoi(secStr)
	if err != nil {
		return
	}
	err = CheckRange(&secId, a.prefix+"/sections/id", len(a.sections))
	if err != nil {
		return
	}
	section = a.sections[secId]
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

type MQTTUpdater struct {
	sections        []Section
	programs        []Program
	onSectionUpdate chan SecUpdate
	onProgramUpdate chan ProgUpdate
	stop            chan int
	api             *MQTTApi
	logger          *logrus.Entry
}

func (a *MQTTApi) UpdateSectionData(index int, sec Section) (err error) {
	bytes, err := json.Marshal(sec)
	if err != nil {
		err = fmt.Errorf("error marshalling section: %v", err)
		return
	}
	a.client.Publish(fmt.Sprintf("%s/sections/%d", a.prefix, index), 1, true, bytes)
	return
}

func (a *MQTTApi) UpdateSectionState(index int, sec Section) (err error) {
	bytes := []byte(strconv.FormatBool(sec.State()))
	a.client.Publish(fmt.Sprintf("%s/sections/%d/state", a.prefix, index), 1, true, bytes)
	return
}

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

func (a *MQTTApi) UpdateProgramRunning(index int, prog *Program) (err error) {
	bytes := []byte(strconv.FormatBool(prog.Running()))
	a.client.Publish(fmt.Sprintf("%s/programs/%d/running", a.prefix, index), 1, true, bytes)
	return
}

func (a *MQTTApi) UpdatePrograms(programs []Program) (err error) {
	lenPrograms := len(programs)
	bytes := []byte(strconv.Itoa(lenPrograms))
	a.client.Publish(a.prefix+"/programs", 1, true, bytes)
	for i, _ := range programs {
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

func NewMQTTUpdater(sections []Section, programs []Program) *MQTTUpdater {
	onSectionUpdate, onProgramUpdate, stop := make(chan SecUpdate, 10), make(chan ProgUpdate, 10), make(chan int)
	for i, _ := range sections {
		sections[i].SetOnUpdate(onSectionUpdate)
	}
	for i, _ := range programs {
		programs[i].OnUpdate = onProgramUpdate
	}
	return &MQTTUpdater{
		sections, programs,
		onSectionUpdate, onProgramUpdate, stop, nil,
		Logger.WithField("module", "MQTTUpdater"),
	}
}

func (u *MQTTUpdater) UpdateSections() {
	u.api.UpdateSections(u.sections)
}

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
			case SEC_UPDATE_DATA:
				err = u.api.UpdateSectionData(index, secUpdate.Sec)
			case SEC_UPDATE_STATE:
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
			for i, _ := range u.programs {
				if &u.programs[i] == progUpdate.Prog {
					index = i
				}
			}
			if index == -1 {
				u.logger.Panicf("invalid program update recieved: %v", progUpdate.Prog)
			}

			var err error
			switch progUpdate.Type {
			case PROG_UPDATE_DATA:
				err = u.api.UpdateProgramData(index, progUpdate.Prog)
			case PROG_UPDATE_RUNNING:
				err = u.api.UpdateProgramRunning(index, progUpdate.Prog)
			default:
			}
			if err != nil {
				u.logger.WithError(err).Error("error updating sections")
			}
		}
	}
}

func (u *MQTTUpdater) Start(api *MQTTApi) {
	u.api = api
	go u.run()
}

func (u *MQTTUpdater) Stop() {
	u.stop <- 0
}
