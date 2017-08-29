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
	t     *testing.T
	mock.Mock
}

func newMockSection(name string, t *testing.T) *MockSection {
	return &MockSection{false, name, t, mock.Mock{}}
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

func (m *MockSection) SetupReturns() {
	m.On("SetState", true).Return()
	m.On("SetState", false).Return()
}

func (m *MockSection) AssertRunning() {
	assert.True(m.t, m.State(), "Section %s should be running", m.name)
}

func (m *MockSection) AssertNotRunning() {
	assert.False(m.t, m.State(), "Section %s should not be running", m.name)
}

func (m *MockSection) AssertAllCalled() {
	m.AssertExpectations(m.t)
}

var _ Section = (*MockSection)(nil)

func TestSectionRun_String(t *testing.T) {
	sec := newMockSection("sec", t)
	sr := NewSectionRun(0, sec, 1*time.Second, nil)
	assert.Equal(t, "{'sec' for 1s}", sr.String())
}

type SRQueueSuite struct {
	suite.Suite
	a     *assert.Assertions
	queue SRQueue
	sec1  *MockSection
	sec2  *MockSection
	sec3  *MockSection
}

func (s *SRQueueSuite) SetupSuite() {
	s.a = assert.New(s.T())
	s.sec1 = newMockSection("mock 1", s.T())
	s.sec2 = newMockSection("mock 2", s.T())
	s.sec3 = newMockSection("mock 3", s.T())
}

func (s *SRQueueSuite) SetupTest() {
	s.queue = newSRQueue(2)
}

func (s *SRQueueSuite) TestPushPop() {
	ass := s.a
	queue := s.queue

	item1 := NewSectionRun(0, s.sec1, 5*time.Second, nil)
	item2 := NewSectionRun(0, s.sec2, 10*time.Second, nil)
	item3 := NewSectionRun(0, s.sec3, 15*time.Second, nil)

	ass.Nil(queue.Pop(), "Pop() should be nil when empty")
	ass.Equal(0, queue.Len(), "Len() should be 0 when empty")

	queue.Push(&item1)
	ass.Equal(1, queue.Len(), "Len() does not match")
	queue.Push(&item2)
	ass.Equal(2, queue.Len(), "Len() does not match")
	queue.Push(&item3)
	ass.Equal(3, queue.Len(), "Len() does not match")

	ass.Equal(&item1, queue.Pop(), "item1 is not 1 out of queue")
	ass.Equal(&item2, queue.Pop(), "item2 is not 2 out of queue")
	ass.Equal(&item3, queue.Pop(), "item3 is not 3 out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")
}

func (s *SRQueueSuite) TestOverflow() {
	ass := s.a
	queue := s.queue

	item1 := NewSectionRun(0, s.sec1, 5*time.Second, nil)
	item2 := NewSectionRun(0, s.sec2, 10*time.Second, nil)
	item3 := NewSectionRun(0, s.sec3, 15*time.Second, nil)

	queue.Push(&item1)
	ass.Equal(1, queue.Len(), "Len() does not match")
	ass.Equal(&item1, queue.Pop(), "item1 is not out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")
	queue.Push(&item2)
	ass.Equal(1, queue.Len(), "Len() does not match")
	ass.Equal(&item2, queue.Pop(), "item2 is not out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")
	queue.Push(&item3)
	ass.Equal(1, queue.Len(), "Len() does not match")
	ass.Equal(&item3, queue.Pop(), "item3 is not out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")

}

func (s *SRQueueSuite) TestNilPop() {
	ass := s.a
	queue := s.queue

	item1 := NewSectionRun(0, s.sec1, 5*time.Second, nil)
	item2 := NewSectionRun(0, s.sec2, 10*time.Second, nil)

	queue.Push(&item1)
	queue.Push(nil)
	ass.Equal(1, queue.Len(), "Len() does not match")
	queue.Push(&item2)
	ass.Equal(2, queue.Len(), "Len() does not match")
	ass.Equal(&item1, queue.Pop(), "item1 is not out of queue")
	ass.Equal(&item2, queue.Pop(), "item2 is not out of queue")
}

func (s *SRQueueSuite) TestRemove() {
	ass := s.a
	queue := s.queue

	item1 := NewSectionRun(0, s.sec1, 5*time.Second, nil)
	item2 := NewSectionRun(0, s.sec2, 10*time.Second, nil)
	item3 := NewSectionRun(0, s.sec3, 15*time.Second, nil)

	queue.Push(&item1)
	queue.Push(&item2)
	queue.Push(&item3)
	ass.Equal(3, queue.Len(), "Len() does not match")

	queue.RemoveMatchingSection(s.sec2)
	ass.Equal(2, queue.Len(), "Len() does not match")
	queue.RemoveMatchingSection(s.sec1)
	ass.Equal(1, queue.Len(), "Len() does not match")
	ass.Equal(&item3, queue.Pop(), "item3 is not 3 out of queue")
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
	secs []Section
}

func (s *SectionRunnerSuite) SetupSuite() {
	s.ass = assert.New(s.T())
}

func (s *SectionRunnerSuite) SetupTest() {
	Logger.Out = ioutil.Discard
	s.sec1 = newMockSection("mock 1", s.T())
	s.sec2 = newMockSection("mock 2", s.T())
	s.secs = []Section{s.sec1, s.sec2}
	s.sr = NewSectionRunner()
	s.sr.Start(nil)
	s.ass.NotNil(s.sr)
}

func (s *SectionRunnerSuite) TearDownTest() {
	s.sr.Quit()
}

func (s *SectionRunnerSuite) TestRunSection() {
	s.sec1.On("SetState", true).
		Return().
		Run(func(args mock.Arguments) {
			s.sec1.On("SetState", false).Return()
		})

	s.sr.RunSection(s.sec1, 10*time.Nanosecond)
	s.sec1.AssertAllCalled()
	s.sec2.AssertAllCalled()
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
	s.sec1.AssertAllCalled()
	s.sec2.AssertAllCalled()
}

func (s *SectionRunnerSuite) TestStateToJSON() {
	s.sec1.SetupReturns()

	s.sr.QueueSectionRun(s.sec1, time.Minute)
	s.sr.QueueSectionRun(s.sec2, time.Minute)
	time.Sleep(10 * time.Millisecond)

	_, err := s.sr.State.ToJSON([]Section{s.sec1})
	s.ass.Error(err, "should error because passed sections doesn't contian s.sec2")
}

func (s *SectionRunnerSuite) TestRunAsync() {
	s.sec1.SetupReturns()
	s.sec1.AssertNotRunning()
	_, c := s.sr.RunSectionAsync(s.sec1, 50*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	s.sec1.AssertRunning()
	
	s.sr.State.Lock()
	json, err := s.sr.State.ToJSON(s.secs)
	s.sr.State.Unlock()
	s.ass.NoError(err)
	s.ass.Equal(0, json.Current.Section)
	s.ass.Empty(json.Queue)

	<-c
	s.sec1.AssertNotRunning()

	s.sr.State.Lock()
	json, err = s.sr.State.ToJSON(s.secs)
	s.sr.State.Unlock()
	s.ass.NoError(err)
	s.ass.Nil(json.Current)
	s.ass.Empty(json.Queue)

	s.sec1.AssertAllCalled()
	s.sec2.AssertAllCalled()
}

func (s *SectionRunnerSuite) TestCancelSection() {
	s.sec1.SetupReturns()
	s.sec2.SetupReturns()

	s.sr.QueueSectionRun(s.sec1, time.Minute)
	s.sr.QueueSectionRun(s.sec2, time.Minute)

	time.Sleep(10 * time.Millisecond)
	json, _ := s.sr.State.ToJSON(s.secs)
	s.ass.Len(json.Queue, 1, "There should be 1 item in the queue")
	s.ass.Equal(1, json.Queue[0].Section)
	s.ass.Equal(60.0, json.Queue[0].Duration)
	s.ass.Equal(0, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)
	s.sec2.AssertNotRunning()
	s.sec1.AssertRunning()

	s.sr.CancelSection(s.sec2)
	time.Sleep(10 * time.Millisecond)
	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.Empty(json.Queue, "There should be 0 items in the queue")
	s.ass.Equal(0, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)
	s.sec2.AssertNotRunning()
	s.sec1.AssertRunning()

	s.sr.QueueSectionRun(s.sec2, time.Minute)
	s.sr.CancelSection(s.sec1)
	time.Sleep(10 * time.Millisecond)
	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.Empty(json.Queue, "There should be 0 items in the queue")
	s.ass.Equal(1, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)
	s.sec2.AssertRunning()
	s.sec1.AssertNotRunning()

	s.sr.CancelSection(s.sec2)
	time.Sleep(10 * time.Millisecond)

	s.sec1.AssertAllCalled()
	s.sec2.AssertAllCalled()
}

func (s *SectionRunnerSuite) TestCancelID() {
	s.sec1.SetupReturns()
	s.sec2.SetupReturns()

	id1 := s.sr.QueueSectionRun(s.sec1, time.Minute)
	id2 := s.sr.QueueSectionRun(s.sec2, time.Minute)

	time.Sleep(10 * time.Millisecond)
	json, _ := s.sr.State.ToJSON(s.secs)
	s.ass.Len(json.Queue, 1, "There should be 1 item in the queue")
	s.ass.Equal(1, json.Queue[0].Section)
	s.ass.Equal(60.0, json.Queue[0].Duration)
	s.ass.Equal(0, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)
	s.sec2.AssertNotRunning()
	s.sec1.AssertRunning()

	s.sr.CancelID(id2)
	time.Sleep(10 * time.Millisecond)
	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.Empty(json.Queue, "There should be 0 items in the queue")
	s.ass.Equal(0, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)
	s.sec2.AssertNotRunning()
	s.sec1.AssertRunning()

	id2 = s.sr.QueueSectionRun(s.sec2, time.Minute)
	s.sr.CancelID(id1)
	time.Sleep(10 * time.Millisecond)
	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.Empty(json.Queue, "There should be 0 items in the queue")
	s.ass.Equal(1, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)
	s.sec2.AssertRunning()
	s.sec1.AssertNotRunning()

	s.sr.CancelID(id2)
	time.Sleep(10 * time.Millisecond)

	s.sec1.AssertAllCalled()
	s.sec2.AssertAllCalled()
}

func (s *SectionRunnerSuite) TestPause() {
	s.sec1.SetupReturns()
	s.sec2.SetupReturns()

	id1 := s.sr.QueueSectionRun(s.sec1, time.Minute)
	time.Sleep(10 * time.Millisecond)
	s.ass.True(s.sec1.State(), "Section should be running")

	json, _ := s.sr.State.ToJSON(s.secs)
	s.ass.Equal(0, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)

	s.sr.Pause()
	time.Sleep(10 * time.Millisecond)
	s.ass.True(s.sr.State.Paused, "SectionRunner should be paused")
	s.ass.False(s.sec1.State(), "Section should not be running")

	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.True(json.Paused)
	s.ass.Equal(0, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)
	s.ass.NotNil(json.Current.PauseTime)

	s.sr.Pause() // double pause should change nothing
	time.Sleep(10 * time.Millisecond)
	s.ass.True(s.sr.State.Paused, "SectionRunner should be paused")
	s.ass.False(s.sec1.State(), "Section should not be running")

	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.True(json.Paused)
	s.ass.Equal(0, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)
	s.ass.NotNil(json.Current.PauseTime)

	s.sr.Unpause()
	time.Sleep(10 * time.Millisecond)
	s.ass.False(s.sr.State.Paused, "SectionRunner should not be paused")
	s.ass.True(s.sec1.State(), "Section should be running")
	
	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.False(json.Paused)
	s.ass.Equal(0, json.Current.Section)
	s.ass.Equal(60.0, json.Current.Duration)
	s.ass.Nil(json.Current.PauseTime)

	s.sr.QueueSectionRun(s.sec2, 40*time.Millisecond)
	s.sr.Pause()
	s.sr.CancelID(id1)
	time.Sleep(10 * time.Millisecond)
	s.ass.True(s.sr.State.Paused, "SectionRunner should be paused")
	s.ass.False(s.sec1.State(), "Section should not be running")
	s.ass.False(s.sec2.State(), "Section should not be running")

	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.True(json.Paused)
	s.ass.Equal(1, json.Current.Section)
	s.ass.Equal(0.04, json.Current.Duration)
	s.ass.NotNil(json.Current.PauseTime)

	s.sr.Unpause()
	time.Sleep(20 * time.Millisecond)
	s.ass.True(s.sec2.State(), "Section should be running")

	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.False(json.Paused)
	s.ass.Equal(1, json.Current.Section)
	s.ass.Equal(0.04, json.Current.Duration)
	s.ass.Nil(json.Current.PauseTime)

	s.sr.Pause()
	time.Sleep(10 * time.Millisecond)
	s.ass.False(s.sec2.State(), "Section should not be running")

	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.True(json.Paused)
	s.ass.Equal(1, json.Current.Section)
	s.ass.Equal(0.04, json.Current.Duration)
	s.ass.NotNil(json.Current.PauseTime)

	s.sr.Unpause()
	time.Sleep(30 * time.Millisecond)
	// It should have started for 20 ms, then paused for 10 ms, then run again for 30 ms.
	// So 20ms + 30ms > 40ms, should be done
	s.ass.False(s.sec2.State(), "Section should not be running")

	json, _ = s.sr.State.ToJSON(s.secs)
	s.ass.False(json.Paused)
	s.ass.Nil(json.Current)

	s.sec1.AssertAllCalled()
	s.sec2.AssertAllCalled()
}

func TestSectionRunner(t *testing.T) {
	suite.Run(t, new(SectionRunnerSuite))
}
