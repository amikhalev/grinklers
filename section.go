package grinklers

import (
	"encoding/json"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/stianeikeland/go-rpio"
	"os"
)

type Section interface {
	SetState(on bool)
	State() (on bool)
	SetOnUpdate(chan<- Section)
	Name() string
}

var (
	RPI           bool
	sectionValues []bool
)

func InitSection() {
	RPI = (os.Getenv("RPI") == "true")
	if RPI {
		Logger.Info("opening rpio")
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
	onUpdate chan<- Section
	log15.Logger
}

var _ Section = (*RpioSection)(nil)

func NewRpioSection(name string, pin rpio.Pin) RpioSection {
	return RpioSection{
		name, pin,
		nil,
		Logger.New("section", name),
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

func (sec *RpioSection) SetOnUpdate(onUpdate chan<- Section) {
	sec.onUpdate = onUpdate
}

func (sec *RpioSection) update() {
	if sec.onUpdate != nil {
		sec.onUpdate <- sec
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
	sec.update()
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

type RpioSections []Section

func (secs *RpioSections) UnmarshalJSON(b []byte) (err error) {
	var s []RpioSection
	err = json.Unmarshal(b, &s)
	if err != nil {
		return
	}
	*secs = make(RpioSections, len(s))
	for i, _  := range s {
		(*secs)[i] = &s[i]
	}
	return
}

var _ json.Unmarshaler = (*RpioSections)(nil)