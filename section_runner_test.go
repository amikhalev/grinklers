package grinklers

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MockSection struct {
	state bool
	name  string
	mock.Mock
}

func newMockSection(name string) *MockSection {
	return &MockSection{false, name, mock.Mock{}}
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

func (m *MockSection) Name() string {
	return m.name
}

var _ Section = (*MockSection)(nil)

func TestSectionRun_String(t *testing.T) {
	sec := newMockSection("sec")
	sr := SectionRun{sec, 1 * time.Second, nil, nil}
	assert.Equal(t, "{'sec' for 1s}", sr.String())
}

type SRQueueSuite struct {
	suite.Suite
	a     *assert.Assertions
	queue SrQueue
	sec1  *MockSection
	sec2  *MockSection
	sec3  *MockSection
}

func (s *SRQueueSuite) SetupSuite() {
	s.a = assert.New(s.T())
	s.sec1 = newMockSection("mock 1")
	s.sec2 = newMockSection("mock 2")
	s.sec3 = newMockSection("mock 3")
}

func (s *SRQueueSuite) SetupTest() {
	s.queue = newSRQueue(2)
}

func (s *SRQueueSuite) TestPushPop() {
	ass := s.a
	queue := s.queue

	item1 := &SectionRun{s.sec1, 5 * time.Second, nil, nil}
	item2 := &SectionRun{s.sec2, 10 * time.Second, nil, nil}
	item3 := &SectionRun{s.sec3, 15 * time.Second, nil, nil}

	ass.Nil(queue.Pop(), "Pop() should be nil when empty")
	ass.Equal(0, queue.Len(), "Len() should be 0 when empty")

	queue.Push(item1)
	ass.Equal(1, queue.Len(), "Len() does not match")
	queue.Push(item2)
	ass.Equal(2, queue.Len(), "Len() does not match")
	queue.Push(item3)
	ass.Equal(3, queue.Len(), "Len() does not match")

	ass.Equal(item1, queue.Pop(), "item1 is not 1 out of queue")
	ass.Equal(item2, queue.Pop(), "item2 is not 2 out of queue")
	ass.Equal(item3, queue.Pop(), "item3 is not 3 out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")
}

func (s *SRQueueSuite) TestOverflow() {
	ass := s.a
	queue := s.queue

	item1 := &SectionRun{s.sec1, 5 * time.Second, nil, nil}
	item2 := &SectionRun{s.sec2, 10 * time.Second, nil, nil}
	item3 := &SectionRun{s.sec3, 15 * time.Second, nil, nil}

	queue.Push(item1)
	ass.Equal(1, queue.Len(), "Len() does not match")
	ass.Equal(item1, queue.Pop(), "item1 is not out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")
	queue.Push(item2)
	ass.Equal(1, queue.Len(), "Len() does not match")
	ass.Equal(item2, queue.Pop(), "item2 is not out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")
	queue.Push(item3)
	ass.Equal(1, queue.Len(), "Len() does not match")
	ass.Equal(item3, queue.Pop(), "item3 is not out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")

}

func (s *SRQueueSuite) TestNilPop() {
	ass := s.a
	queue := s.queue

	item1 := &SectionRun{s.sec1, 5 * time.Second, nil, nil}
	item2 := &SectionRun{s.sec2, 10 * time.Second, nil, nil}

	queue.Push(item1)
	queue.Push(nil)
	ass.Equal(1, queue.Len(), "Len() does not match")
	queue.Push(item2)
	ass.Equal(2, queue.Len(), "Len() does not match")
	ass.Equal(item1, queue.Pop(), "item1 is not out of queue")
	ass.Equal(item2, queue.Pop(), "item2 is not out of queue")
}

func (s *SRQueueSuite) TestRemove() {
	ass := s.a
	queue := s.queue

	item1 := &SectionRun{s.sec1, 5 * time.Second, nil, nil}
	item2 := &SectionRun{s.sec2, 10 * time.Second, nil, nil}
	item3 := &SectionRun{s.sec3, 15 * time.Second, nil, nil}

	queue.Push(item1)
	queue.Push(item2)
	queue.Push(item3)
	ass.Equal(3, queue.Len(), "Len() does not match")

	queue.RemoveMatchingSection(s.sec2)
	ass.Equal(2, queue.Len(), "Len() does not match")
	queue.RemoveMatchingSection(s.sec1)
	ass.Equal(1, queue.Len(), "Len() does not match")
	ass.Equal(item3, queue.Pop(), "item3 is not 3 out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")
}

func TestSRQueue(t *testing.T) {
	suite.Run(t, new(SRQueueSuite))
}

type SectionRunnerSuite struct {
	suite.Suite
	ass  *assert.Assertions
	sr   *SectionRunner
	sec1 *MockSection
	sec2 *MockSection
}

func (s *SectionRunnerSuite) SetupSuite() {
	s.ass = assert.New(s.T())
}

func (s *SectionRunnerSuite) SetupTest() {
	Logger.Out = ioutil.Discard
	s.sec1 = newMockSection("mock 1")
	s.sec2 = newMockSection("mock 2")
	s.sr = NewSectionRunner()
	s.sr.Start(nil)
	s.ass.NotNil(s.sr)
}

func (s *SectionRunnerSuite) TearDownTest() {
	s.sec1.AssertExpectations(s.T())
	s.sec2.AssertExpectations(s.T())
	s.sr.Quit()
}

func (s *SectionRunnerSuite) TestRunSection() {
	s.sec1.On("SetState", true).
		Return().
		Run(func(args mock.Arguments) {
			s.sec1.On("SetState", false).Return()
		})

	s.sr.RunSection(s.sec1, 10*time.Nanosecond)
}

func (s *SectionRunnerSuite) TestSectionQueue() {
	s.sec1.On("SetState", true).Return().
		Run(func(args mock.Arguments) {
			s.sec1.On("SetState", false).Return()
			s.sec2.On("SetState", true).Return().
				Run(func(args mock.Arguments) {
					s.sec2.On("SetState", false).Return()
				})
		})

	s.sr.QueueSectionRun(s.sec1, 10*time.Nanosecond)
	s.sr.QueueSectionRun(s.sec2, 10*time.Nanosecond)
	time.Sleep(50 * time.Millisecond)
}

func (s *SectionRunnerSuite) TestRunAsync() {
	s.sec1.On("SetState", true).Return()
	s.sec1.On("SetState", false).Return()
	s.ass.Equal(false, s.sec1.State(), "sec1 should be off")
	c := s.sr.RunSectionAsync(s.sec1, 50*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	s.ass.Equal(true, s.sec1.State(), "sec1 should be on")
	<-c
	s.ass.Equal(false, s.sec1.State(), "sec1 should be off")
}

func (s *SectionRunnerSuite) TestCancel() {
	s.sec1.On("SetState", true).Return()
	s.sec1.On("SetState", false).Return()

	s.sr.QueueSectionRun(s.sec1, time.Minute)
	s.sr.QueueSectionRun(s.sec2, time.Minute)
	time.Sleep(100 * time.Millisecond)
	s.sr.CancelSection(s.sec2)
	s.sr.CancelSection(s.sec1)
	time.Sleep(50 * time.Millisecond)

	s.sec2.AssertNotCalled(s.T(), "SetState", true)
	s.sec2.AssertNotCalled(s.T(), "SetState", false)
}

func TestSectionRunner(t *testing.T) {
	suite.Run(t, new(SectionRunnerSuite))
}
