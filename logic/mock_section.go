package logic

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

type MockSection struct {
	id    int
	state bool
	name  string
	t     *testing.T
	mock.Mock
}

func NewMockSection(id int, name string, t *testing.T) *MockSection {
	return &MockSection{id, false, name, t, mock.Mock{}}
}

func (m *MockSection) SetState(on bool) {
	m.Called(on)
	m.state = on
}

func (m *MockSection) State() bool {
	return m.state
}

func (m *MockSection) SetOnUpdate(onUpdate chan<- SecUpdate) {
	m.Called(onUpdate)
}

func (m *MockSection) ID() int {
	return m.id
}

func (m *MockSection) Name() string {
	return m.name
}

func (m *MockSection) SetupReturns() {
	m.On("SetState", true).Return()
	m.On("SetState", false).Return()
}
