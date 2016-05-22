package main

import (
	"github.com/stianeikeland/go-rpio"
	log "github.com/inconshreveable/log15"
	"time"
	"os"
	"encoding/json"
)

var (
	RPI bool
	sectionValues []bool
)

func init() {
	RPI = (os.Getenv("RPI") == "true")
	if (RPI) {
		log.Info("opening rpio")
		err := rpio.Open()
		if err != nil {
			log.Error("error opening rpio", "err", err)
			os.Exit(1)
		}
	} else {
		sectionValues = make([]bool, 24)
	}
}

func Cleanup() {
	rpio.Close()
}

type Section struct {
	Name     string
	pin      rpio.Pin
	OnUpdate chan *Section
	log.Logger
}

func NewSection(name string, pin rpio.Pin) Section {
	return Section{
		name, pin,
		nil,
		log.New("section", name),
	}
}

type sectionJson struct {
	Name  string `json:"name"`
	Pin   rpio.Pin `json:"pin"`
	Value rpio.State `json:"value"`
}

func (sec *Section) UnmarshalJSON(b []byte) (err error) {
	var d sectionJson
	if err = json.Unmarshal(b, &d); err == nil {
		*sec = NewSection(d.Name, d.Pin)
	}
	return
}

func (sec *Section) MarshalJSON() ([]byte, error) {
	d := sectionJson{
		sec.Name, sec.pin, sec.Value(),
	}
	return json.Marshal(&d)
}

func (sec *Section) onUpdate() {
	if sec.OnUpdate != nil {
		sec.Debug("sending update")
		sec.OnUpdate <- sec
	} else {
		sec.Debug("OnUpdate nil!")
	}
}

func (sec *Section) On() {
	if (RPI) {
		sec.Debug("turning gpio on")
		sec.pin.Output()
		sec.pin.High()
	} else {
		sec.Debug("[stub] section on")
		sectionValues[sec.pin - 2] = true
	}
	sec.onUpdate()
}

func (sec *Section) Off() {
	if (RPI) {
		sec.Debug("turning gpio off")
		sec.pin.Low()
		sec.pin.Input()
	} else {
		sec.Debug("[stub] section off")
		sectionValues[sec.pin - 2] = false
	}
	sec.onUpdate()
}

func (sec *Section) Value() rpio.State {
	if (RPI) {
		return sec.pin.Read()
	} else {
		if sectionValues[sec.pin - 2] {
			return rpio.High
		} else {
			return rpio.Low
		}
	}
}

func (s *Section) Cancel() {
	sectionRunner.CancelSection(s)
}

func (s *Section) RunForAsync(dur time.Duration) (<-chan int) {
	return sectionRunner.RunSectionAsync(s, dur)
}

func (s *Section) RunFor(dur time.Duration) {
	<-s.RunForAsync(dur)
}