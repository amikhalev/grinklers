package main

import (
	"encoding/json"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/stianeikeland/go-rpio"
	"os"
	"time"
)

type Section interface {
	SetState(on bool)
	State() (on bool)
	Name() string
}

var (
	RPI           bool
	sectionValues []bool
)

func InitSection() {
	RPI = (os.Getenv("RPI") == "true")
	if RPI {
		logger.Info("opening rpio")
		err := rpio.Open()
		if err != nil {
			panic(fmt.Errorf("error opening rpio: %v", err))
		}
	} else {
		sectionValues = make([]bool, 24)
	}
}

func CleanupSection() {
	if RPI {
		rpio.Close()
	} else {
		sectionValues = nil
	}
}

type RpioSection struct {
	name     string
	pin      rpio.Pin
	OnUpdate chan<- *RpioSection
	log15.Logger
}

var _ Section = (*RpioSection)(nil)

func NewRpioSection(name string, pin rpio.Pin) RpioSection {
	return RpioSection{
		name, pin,
		nil,
		logger.New("section", name),
	}
}

type rpioSectionJson struct {
	Name  string   `json:"name"`
	Pin   rpio.Pin `json:"pin"`
	State bool     `json:"state"`
}

func (sec *RpioSection) UnmarshalJSON(b []byte) (err error) {
	var d rpioSectionJson
	if err = json.Unmarshal(b, &d); err == nil {
		*sec = NewRpioSection(d.Name, d.Pin)
	}
	return
}

func (sec *RpioSection) MarshalJSON() ([]byte, error) {
	d := rpioSectionJson{
		sec.name, sec.pin, sec.State(),
	}
	return json.Marshal(&d)
}

func (sec *RpioSection) onUpdate() {
	if sec.OnUpdate != nil {
		sec.OnUpdate <- sec
	}
}

func (sec *RpioSection) SetState(on bool) {
	if RPI {
		sec.Debug("setting section state", "on", on)
		if on {
			sec.pin.Output()
			sec.pin.High()
		} else {
			sec.pin.Low()
			sec.pin.Input()
		}
	} else {
		sec.Debug("[stub] setting section state", "on", on)
		sectionValues[sec.pin-2] = on
	}
	sec.onUpdate()
}

func (sec *RpioSection) State() bool {
	if RPI {
		return sec.pin.Read() == rpio.High
	} else {
		return sectionValues[sec.pin-2]
	}
}

func (sec *RpioSection) Name() string {
	return sec.name
}

func (s *RpioSection) Cancel() {
	sectionRunner.CancelSection(s)
}

func (s *RpioSection) RunForAsync(dur time.Duration) <-chan int {
	return sectionRunner.RunSectionAsync(s, dur)
}

func (s *RpioSection) RunFor(dur time.Duration) {
	<-s.RunForAsync(dur)
}
