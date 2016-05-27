package grinklers

import (
	"encoding/json"
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"net/url"
	"os"
	"strconv"
	"time"
)

type apiHandlerFunc func(client mqtt.Client, message mqtt.Message) (resp interface{}, err error)

type respJson struct {
	Rid      int    `json:"rid,omitempty"`
	Response interface{} `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

type MqttApi struct {
	sections  []Section
	programs  []Program
	secRunner *SectionRunner
	client    mqtt.Client
	prefix    string
}

func NewMqttApi(sections []Section, programs []Program, secRunner *SectionRunner) *MqttApi {
	return &MqttApi{
		sections, programs, secRunner,
		nil, "",
	}
}

func createMqttOpts(brokerUri *url.URL, cid string) (opts *mqtt.ClientOptions) {
	opts = mqtt.NewClientOptions()
	opts.AddBroker(brokerUri.String())
	if brokerUri.User != nil {
		username := brokerUri.User.Username()
		opts.SetUsername(username)
		password, _ := brokerUri.User.Password()
		opts.SetPassword(password)
		Logger.Debug("authenticating to mqtt server", "username", username, "password", password)
	}
	opts.SetClientID(cid)
	return
}

func (a *MqttApi) Start() (err error) {
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}
	brokerUri, err := url.Parse(broker)
	if err != nil {
		err = fmt.Errorf("error parsing MQTT_BROKER: %v", err)
		return
	}
	cid := os.Getenv("MQTT_CID")
	if cid == "" {
		cid = "grinklers-1"
	}
	if brokerUri.Path != "" {
		a.prefix = brokerUri.Path
	} else {
		a.prefix = "grinklers"
	}
	Logger.Debug("broker prefix", "prefix", a.prefix)

	opts := createMqttOpts(brokerUri, cid)
	opts.SetWill(a.prefix + "/connected", "false", 1, true)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		Logger.Info("connected to mqtt broker")
		a.updateConnected(true)
	})
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		Logger.Warn("lost connection to mqtt broker", "err", err)
	})
	a.client = mqtt.NewClient(opts)

	if token := a.client.Connect(); token.Wait() && token.Error() != nil {
		Logger.Error("error connecting to mqtt broker", "err", token.Error())
	}

	a.subscribe()

	return
}

func (a *MqttApi) Stop() {
	Logger.Info("disconnecting from mqtt broker")
	a.updateConnected(false)
	a.client.Disconnect(250)
}

func (a *MqttApi) Client() mqtt.Client {
	return a.client
}

func (a *MqttApi) Prefix() string {
	return a.prefix
}

func (a *MqttApi) updateConnected(connected bool) (err error) {
	str := strconv.FormatBool(connected)
	token := a.client.Publish(a.prefix + "/connected", 1, true, str)
	if token.Wait(); token.Error() != nil {
		return token.Error()
	}
	return
}

func (a *MqttApi) subscribeHandler(path string, handler apiHandlerFunc) {
	Logger.Debug("registering handler", "path", a.prefix + path)
	a.client.Subscribe(a.prefix + path, 2, func(client mqtt.Client, message mqtt.Message) {
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

		defer func() {
			if pan := recover(); pan != nil {
				Logger.Warn("panic in api responder", "panic", pan)
				err = fmt.Errorf("internal server panic: %v", pan)
			}
			var rData respJson
			if err != nil {
				Logger.Info("error processing request", "err", err)
				rData = respJson{data.Rid, nil, err.Error()}
			} else {
				rData = respJson{data.Rid, res, ""}
			}
			resBytes, _ := json.Marshal(&rData)
			client.Publish(a.prefix + path + "/response", 1, false, resBytes)
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

func (a *MqttApi) UpdateSections(sections []Section) (err error) {
	bytes, err := json.Marshal(sections)
	if err != nil {
		err = fmt.Errorf("error marshalling sections: %v", err)
		return
	}
	token := a.client.Publish(a.prefix + "/sections", 1, true, bytes)
	if token.Wait(); token.Error() != nil {
		err = fmt.Errorf("error publishing sections: %v", token.Error())
		return
	}
	//logger.Debug("updated sections", "bytes", string(bytes))
	return
}

func (a *MqttApi) UpdatePrograms(programs []Program) (err error) {
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
	token := a.client.Publish(a.prefix + "/programs", 1, true, bytes)
	if token.Wait(); token.Error() != nil {
		err = fmt.Errorf("error publishing programs: %v", token.Error())
		return
	}
	//logger.Debug("updated programs", "bytes", string(bytes))
	return
}

func (a *MqttApi) subscribe() {
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

type MqttUpdater struct {
	sections        []Section
	programs        []Program
	onSectionUpdate chan Section
	onProgramUpdate chan *Program
	stop            chan int
	api             *MqttApi
}

func NewMqttUpdater(sections []Section, programs []Program) *MqttUpdater {
	onSectionUpdate, onProgramUpdate, stop := make(chan Section, 10), make(chan *Program, 10), make(chan int)
	for i, _ := range sections {
		sections[i].SetOnUpdate(onSectionUpdate)
	}
	for i, _ := range programs {
		programs[i].OnUpdate = onProgramUpdate
	}
	return &MqttUpdater{
		sections, programs,
		onSectionUpdate, onProgramUpdate, stop, nil,
	}
}

func (u *MqttUpdater) UpdateSections() (error) {
	return u.api.UpdateSections(u.sections)
}

func (u *MqttUpdater) UpdatePrograms() (error) {
	return u.api.UpdatePrograms(u.programs)
}

func (u *MqttUpdater) run() {
	Logger.Debug("starting updater")
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
				Logger.Error("error updating sections", "err", err)
			}
		case <-u.onProgramUpdate:
		//logger.Debug("prog update")
			ExhaustChan(u.onProgramUpdate)
			err := u.UpdatePrograms()
			if err != nil {
				Logger.Error("error updating programs", "err", err)
			}
		}
	}
}

func (u *MqttUpdater) Start(api *MqttApi) {
	u.api = api
	go u.run()
}

func (u *MqttUpdater) Stop() {
	u.stop <- 0
}