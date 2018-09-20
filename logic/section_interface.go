package logic

type SectionID = uint16

// SectionInterface is an interface implemented by structs which are able to interface
// with hardware for controlling sprinklers section. It is not necessarily backed by
// hardware (as in MockSectionInterface)
type SectionInterface interface {
	Name() string

	Initialize() error
	Deinitialize() error

	Count() SectionID
	Set(sectionNum SectionID, state bool)
	Get(sectionNum SectionID) (state bool)
}
