package logic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockSectionInterface struct {
	states []bool
	mock.Mock
}

var _ SectionInterface = (*MockSectionInterface)(nil)

func NewMockSectionInterface(len int) *MockSectionInterface {
	states := make([]bool, len)
	return &MockSectionInterface{states, mock.Mock{}}
}

func (m *MockSectionInterface) Name() string {
	return "mock"
}

func (m *MockSectionInterface) Initialize() error {
	for i := range m.states {
		m.states[i] = false
	}
	m.ExpectedCalls = nil
	m.Calls = nil
	m.SetupAllReturns()
	return nil
}

func (m *MockSectionInterface) Deinitialize() error {
	return m.Initialize()
}

func (m *MockSectionInterface) Count() SectionID {
	return (SectionID)(len(m.states))
}

func (m *MockSectionInterface) Set(id SectionID, state bool) {
	m.Called(id, state)
	m.states[id] = state
}

func (m *MockSectionInterface) Get(id SectionID) bool {
	return m.states[id]
}

func (m *MockSectionInterface) SetupReturns(sec *Section) {
	m.On("Set", sec.InterfaceID, true).Return()
	m.On("Set", sec.InterfaceID, false).Return()
}

func (m *MockSectionInterface) SetupAllReturns() {
	for i := range m.states {
		m.On("Set", (SectionID)(i), true).Return()
		m.On("Set", (SectionID)(i), false).Return()
	}
}

func (m *MockSectionInterface) AssertRunning(t *testing.T, sec *Section) {
	assert.True(t, sec.GetState(m), "Section %s should be running", sec.Name)
}

func (m *MockSectionInterface) AssertNotRunning(t *testing.T, sec *Section) {
	assert.False(t, sec.GetState(m), "Section %s should not be running", sec.Name)
}

func (m *MockSectionInterface) AssertAllCalled(t *testing.T) {
	m.AssertExpectations(t)
}
