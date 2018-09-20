package logic

import (
	"fmt"

	"git.amikhalev.com/amikhalev/grinklers/util"
	"github.com/Sirupsen/logrus"
	"github.com/stianeikeland/go-rpio"
)

type RpioPins []rpio.Pin

// RpioSectionInterface is a section interface which uses raspberry pi gpio pins to control sections
type RpioSectionInterface struct {
	pins RpioPins
	log  *logrus.Entry
}

var _ SectionInterface = (*RpioSectionInterface)(nil)

func NewRpioSectionInterface(pins RpioPins) *RpioSectionInterface {
	return &RpioSectionInterface{
		pins,
		util.Logger.WithField("section_interface", "rpio"),
	}
}

func (i *RpioSectionInterface) Name() string {
	return "rpio"
}

func (i *RpioSectionInterface) Initialize() (err error) {
	i.log.Info("opening rpio")
	err = rpio.Open()
	if err != nil {
		err = fmt.Errorf("error opening rpio: %v", err)
	}
	return
}

func (i *RpioSectionInterface) Deinitialize() (err error) {
	return rpio.Close()
}

func (i *RpioSectionInterface) Count() SectionID {
	return (SectionID)(len(i.pins))
}

func (i *RpioSectionInterface) Set(id SectionID, state bool) {
	i.log.WithField("state", state).Debug("setting section state")
	pin := i.pins[id]
	if state {
		pin.Output()
		pin.High()
	} else {
		pin.Low()
		pin.Input()
	}
}

func (i *RpioSectionInterface) Get(id SectionID) bool {
	pin := i.pins[id]
	return pin.Read() == rpio.High
}
