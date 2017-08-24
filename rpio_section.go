package grinklers

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/stianeikeland/go-rpio"
)

var (
	rpi           bool
	sectionValues []bool
)

// RpioSectionInit initializes raspi section functionality and must be called before RpioSections are used
// This reads the "RPI" environment variable to determine if raspi sections should be used. If this environment
// variable is false, mock sections which do not actually use io pins are used.
func RpioSectionInit() (err error) {
	rpi = (os.Getenv("RPI") == "true")
	if rpi {
		Logger.Info("opening rpio")
		err = rpio.Open()
		if err != nil {
			err = fmt.Errorf("error opening rpio: %v", err)
			return
		}
	} else {
		sectionValues = make([]bool, 24)
	}
	return
}

// RpioSectionCleanup uninitializes raspi section functionality and should be called sometime after RpioSectionInit
func RpioSectionCleanup() (err error) {
	if rpi {
		err = rpio.Close()
		if err != nil {
			return
		}
	} else {
		sectionValues = nil
	}
	return
}

// RpioSection is a section which uses raspberry pi io pins to control sections,
// unless rpi is set to false.
type RpioSection struct {
	name     string
	pin      rpio.Pin
	onUpdate chan<- SecUpdate
	log      *logrus.Entry
}

var _ Section = (*RpioSection)(nil)

// NewRpioSection creates a new RpioSection with the specified data
func NewRpioSection(name string, pin rpio.Pin) RpioSection {
	return RpioSection{
		name, pin,
		nil,
		Logger.WithField("section", name),
	}
}

type rpioSectionJSON struct {
	Name string   `json:"name"`
	Pin  rpio.Pin `json:"pin"`
}

// UnmarshalJSON converts JSON to a RpioSection
func (sec *RpioSection) UnmarshalJSON(b []byte) (err error) {
	var d rpioSectionJSON
	if err = json.Unmarshal(b, &d); err == nil {
		*sec = NewRpioSection(d.Name, d.Pin)
	}
	return
}

// MarshalJSON converts a RpioSection to JSON
func (sec *RpioSection) MarshalJSON() ([]byte, error) {
	d := rpioSectionJSON{
		sec.name, sec.pin,
	}
	return json.Marshal(&d)
}

// SetOnUpdate sets the update handler chan for this Section
func (sec *RpioSection) SetOnUpdate(onUpdate chan<- SecUpdate) {
	sec.onUpdate = onUpdate
}

func (sec *RpioSection) update(t SecUpdateType) {
	if sec.onUpdate != nil {
		sec.onUpdate <- SecUpdate{
			Sec: sec, Type: t,
		}
	}
}

// SetState sets the running state of this RpioSection
func (sec *RpioSection) SetState(on bool) {
	if rpi {
		sec.log.WithField("state", on).Debug("setting section state")
		if on {
			sec.pin.Output()
			sec.pin.High()
		} else {
			sec.pin.Low()
			sec.pin.Input()
		}
	} else {
		sec.log.WithField("state", on).Debug("[stub] setting section state")
		sectionValues[sec.pin-2] = on
	}
	sec.update(SecUpdateState)
}

// State gets the current running state of this Section
func (sec *RpioSection) State() bool {
	if !rpi {
		return sectionValues[sec.pin-2]
	}
	return sec.pin.Read() == rpio.High
}

// Name gets the name of this RpioSection
func (sec *RpioSection) Name() string {
	return sec.name
}

// RpioSections represents a list of Sections that are all RpioSections
type RpioSections []Section

// UnmarshalJSON converts JSON to RpioSections
func (secs *RpioSections) UnmarshalJSON(b []byte) (err error) {
	var s []RpioSection
	err = json.Unmarshal(b, &s)
	if err != nil {
		return
	}
	*secs = make(RpioSections, len(s))
	for i := range s {
		(*secs)[i] = &s[i]
	}
	return
}

var _ json.Unmarshaler = (*RpioSections)(nil)
