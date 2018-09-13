package logic

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
	Sec  Section
	Type SecUpdateType
}

// Section is an interface for sprinklers sections which can be turned on and off
type Section interface {
	SetState(on bool)
	State() (on bool)
	SetOnUpdate(chan<- SecUpdate)
	Name() string
	ID() int
}
