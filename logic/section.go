package logic

import "encoding/json"

// SecUpdateType is the type of a SecUpdate
type SecUpdateType int

const (
	// SecUpdateData means a SecUpdate was only to the section data (ie. name)
	SecUpdateData SecUpdateType = iota
	// SecUpdateState means a SecUpdate was only to the section state (ie. Section.State())
	SecUpdateState
)

// SecUpdate is an update made to a Section
type SecUpdate struct {
	Sec  *Section
	Type SecUpdateType
}

// Section is an interface for sprinklers sections which can be turned on and off
type Section struct {
	// ID is the id of the section in the sections array
	ID int `json:"id"`
	// Name is the human readable name of the section
	Name string `json:"name"`
	// InterfaceID is the id of the section used on the SectionInterface
	InterfaceID SectionID `json:"interfaceId"`

	updateChan chan<- SecUpdate
}

func NewSection(id int, name string, interfaceId SectionID) Section {
	return Section{id, name, interfaceId, nil}
}

// SetUpdateChan sets the update handler chan for this Section
func (sec *Section) SetUpdateChan(updateChan chan<- SecUpdate) {
	sec.updateChan = updateChan
}

func (sec *Section) update(t SecUpdateType) {
	if sec.updateChan != nil {
		sec.updateChan <- SecUpdate{
			Sec: sec, Type: t,
		}
	}
}

func (sec *Section) SetState(state bool, secInterface SectionInterface) {
	secInterface.Set(sec.InterfaceID, state)
	sec.update(SecUpdateState)
}

func (sec *Section) GetState(secInterface SectionInterface) (state bool) {
	return secInterface.Get(sec.InterfaceID)
}

// SetState(on bool)
// State() (on bool)
// Name() string
// ID() int

// Sections represents a list of Sections as stored in JSON
type Sections []Section

func (secs *Sections) UnmarshalJSON(b []byte) (err error) {
	var s []Section
	err = json.Unmarshal(b, &s)
	if err != nil {
		return
	}
	for i := range s {
		s[i].ID = i
	}
	*secs = s
	return
}

// var _ json.Unmarshaler = (*RpioSections)(nil)
