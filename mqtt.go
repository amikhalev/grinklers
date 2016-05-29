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

type apiHandlerFunc func(client mqtt.Client, message mqtt.Message) (resp interface{}, err error)

type respJson struct {
	Rid      int         `json:"rid,omitempty"`
	Response interface{} `json:"message,omitempty"`
	Error    string      `json:"error,omitempty"`
}

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
		var data struct {
			Rid int
		}
		var (
			res interface{}
			err error
		)

		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse api request: %v", err)
			return
		}

		defer func() {
			if pan := recover(); pan != nil {
				a.logger.WithField("panic", pan).Warn("panic in api responder")
				err = fmt.Errorf("internal server panic: %v", pan)
			}
			var rData respJson
			if err != nil {
				a.logger.WithError(err).Info("error processing request")
				rData = respJson{data.Rid, nil, err.Error()}
			} else {
				rData = respJson{data.Rid, res, ""}
			}
			resBytes, _ := json.Marshal(&rData)
			client.Publish(path+"/response", 1, false, resBytes)
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

func (a *MQTTApi) UpdateSections(sections []Section) (err error) {
	bytes, err := json.Marshal(sections)
	if err != nil {
		err = fmt.Errorf("error marshalling sections: %v", err)
		return
	}
	token := a.client.Publish(a.prefix+"/sections", 1, true, bytes)
	if token.Wait(); token.Error() != nil {
		err = fmt.Errorf("error publishing sections: %v", token.Error())
		return
	}
	//logger.Debug("updated sections", "bytes", string(bytes))
	return
}

func (a *MQTTApi) UpdatePrograms(programs []Program) (err error) {
	data, err := ProgramsToJSON(programs, a.sections)
	if err != nil {
		err = fmt.Errorf("error converting programs to json: %v", err)
		return
	}
	bytes, err := json.Marshal(&data)
	if err != nil {
		err = fmt.Errorf("error marshalling programs: %v", err)
		return
	}
	token := a.client.Publish(a.prefix+"/programs", 1, true, bytes)
	if token.Wait(); token.Error() != nil {
		err = fmt.Errorf("error publishing programs: %v", token.Error())
		return
	}
	//logger.Debug("updated programs", "bytes", string(bytes))
	return
}

func (a *MQTTApi) subscribe() {
	a.subscribeHandler("/runProgram", func(client mqtt.Client, message mqtt.Message) (res interface{}, err error) {
		data, err := parseProgramJson(message.Payload())
		if err != nil {
			return
		}
		programs := a.programs
		err = CheckRange(data.ProgramId, "programId", len(programs))
		if err != nil {
			return
		}
		program := &programs[*data.ProgramId]
		program.Run()
		res = fmt.Sprintf("running program '%s'", program.Name)
		return
	})

	a.subscribeHandler("/cancelProgram", func(client mqtt.Client, message mqtt.Message) (res interface{}, err error) {
		data, err := parseProgramJson(message.Payload())
		if err != nil {
			return
		}
		programs := a.programs
		err = CheckRange(data.ProgramId, "programId", len(programs))
		if err != nil {
			return
		}
		program := &programs[*data.ProgramId]
		program.Cancel()
		res = fmt.Sprintf("cancelled program '%s'", program.Name)
		return
	})

	a.subscribeHandler("/runSection", func(client mqtt.Client, message mqtt.Message) (res interface{}, err error) {
		var data struct {
			SectionId *int
			Duration  *string
		}
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse run section request: %v", err)
			return
		}
		sections := a.sections
		err = CheckRange(data.SectionId, "sectionId", len(sections))
		if err != nil {
			return
		}
		sec := sections[*data.SectionId]
		duration, err := parseDuration(data.Duration)
		if err != nil {
			return
		}
		done := a.secRunner.RunSectionAsync(sec, *duration)
		go func() {
			<-done
		}()
		res = fmt.Sprintf("running section '%s' for %v", sec.Name(), duration)
		return
	})

	a.subscribeHandler("/cancelSection", func(client mqtt.Client, message mqtt.Message) (res interface{}, err error) {
		var data struct {
			SectionId *int
		}
		err = json.Unmarshal(message.Payload(), &data)
		if err != nil {
			err = fmt.Errorf("could not parse cancel section request: %v", err)
			return
		}
		sections := a.sections
		err = CheckRange(data.SectionId, "sectionId", len(sections))
		if err != nil {
			return
		}
		sec := sections[*data.SectionId]
		a.secRunner.CancelSection(sec)
		res = fmt.Sprintf("cancelled section '%s'", sec.Name())
		return
	})
}

type MQTTUpdater struct {
	sections        []Section
	programs        []Program
	onSectionUpdate chan Section
	onProgramUpdate chan *Program
	stop            chan int
	api             *MQTTApi
	logger          *logrus.Entry
}

func NewMQTTUpdater(sections []Section, programs []Program) *MQTTUpdater {
	onSectionUpdate, onProgramUpdate, stop := make(chan Section, 10), make(chan *Program, 10), make(chan int)
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

func (u *MQTTUpdater) UpdateSections() error {
	return u.api.UpdateSections(u.sections)
}

func (u *MQTTUpdater) UpdatePrograms() error {
	return u.api.UpdatePrograms(u.programs)
}

func (u *MQTTUpdater) run() {
	u.logger.Debug("starting updater")
	for {
		//logger.Debug("waiting for update")
		select {
		case <-u.stop:
			return
		case <-u.onSectionUpdate:
			//logger.Debug("sec update")
			ExhaustChan(u.onSectionUpdate)
			err := u.UpdateSections()
			if err != nil {
				u.logger.WithError(err).Error("error updating sections")
			}
		case <-u.onProgramUpdate:
			//logger.Debug("prog update")
			ExhaustChan(u.onProgramUpdate)
			err := u.UpdatePrograms()
			if err != nil {
				u.logger.WithError(err).Error("error updating programs")
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
