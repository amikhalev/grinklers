package logic

import (
	"io/ioutil"
	"testing"
	"time"

	"git.amikhalev.com/amikhalev/grinklers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func TestSectionRun_String(t *testing.T) {
	sec := NewSection(0, "sec", 0)
	sr := NewSectionRun(0, &sec, 1*time.Second, nil)
	assert.Equal(t, "{'sec' for 1s}", sr.String())
}

type SRQueueSuite struct {
	suite.Suite
	a     *assert.Assertions
	queue SRQueue
	secs  []Section
}

func (s *SRQueueSuite) SetupSuite() {
	s.a = assert.New(s.T())
	s.secs = []Section{
		NewSection(0, "mock 1", 0),
		NewSection(1, "mock 2", 1),
		NewSection(2, "mock 3", 2),
	}
}

func (s *SRQueueSuite) SetupTest() {
	s.queue = NewSRQueue(2)
}

func (s *SRQueueSuite) TestPushPop() {
	ass := s.a
	queue := s.queue

	item1 := NewSectionRun(0, &s.secs[0], 5*time.Second, nil)
	item2 := NewSectionRun(0, &s.secs[1], 10*time.Second, nil)
	item3 := NewSectionRun(0, &s.secs[2], 15*time.Second, nil)

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

	item1 := NewSectionRun(0, &s.secs[0], 5*time.Second, nil)
	item2 := NewSectionRun(0, &s.secs[1], 10*time.Second, nil)
	item3 := NewSectionRun(0, &s.secs[2], 15*time.Second, nil)

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

	item1 := NewSectionRun(0, &s.secs[0], 5*time.Second, nil)
	item2 := NewSectionRun(0, &s.secs[1], 10*time.Second, nil)

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

	item1 := NewSectionRun(0, &s.secs[0], 5*time.Second, nil)
	item2 := NewSectionRun(0, &s.secs[1], 10*time.Second, nil)
	item3 := NewSectionRun(0, &s.secs[2], 15*time.Second, nil)

	queue.Push(&item1)
	queue.Push(&item2)
	queue.Push(&item3)
	ass.Equal(3, queue.Len(), "Len() does not match")

	queue.RemoveWithSection(&s.secs[1])
	ass.Equal(2, queue.Len(), "Len() does not match")
	queue.RemoveWithSection(&s.secs[0])
	ass.Equal(1, queue.Len(), "Len() does not match")
	ass.Equal(&item3, queue.Pop(), "item3 is not 3rd out of queue")
	ass.Equal(0, queue.Len(), "Len() does not match")
}

func (s *SRQueueSuite) TestRemoveAll() {
	ass := s.a
	queue := s.queue

	item1 := NewSectionRun(0, &s.secs[0], 5*time.Second, nil)
	item2 := NewSectionRun(0, &s.secs[1], 10*time.Second, nil)
	item3 := NewSectionRun(0, &s.secs[2], 15*time.Second, nil)

	queue.Push(&item1)
	queue.Push(&item2)
	queue.Push(&item3)
	ass.Equal(3, queue.Len(), "Len() does not match")

	removed := queue.RemoveAll()
	ass.Equal(0, queue.Len(), "Len() does not match")
	ass.Contains(removed, &item1)
	ass.Contains(removed, &item2)
	ass.Contains(removed, &item3)
}

func TestSRQueue(t *testing.T) {
	suite.Run(t, new(SRQueueSuite))
}

type SectionRunnerSuite struct {
	suite.Suite
	ass          *assert.Assertions
	secs         []Section
	secInterface *MockSectionInterface
	sr           *SectionRunner
}

func (s *SectionRunnerSuite) SetupSuite() {
	s.ass = assert.New(s.T())
	s.secs = []Section{NewSection(0, "mock 1", 0), NewSection(1, "mock 2", 1)}
	s.secInterface = NewMockSectionInterface(2)
}

func (s *SectionRunnerSuite) SetupTest() {
	util.Logger.Out = ioutil.Discard
	s.secInterface.Initialize()
	s.secInterface.ExpectedCalls = nil
	s.sr = NewSectionRunner(s.secInterface)
	s.sr.Start(nil)
	s.ass.NotNil(s.sr)
}

func (s *SectionRunnerSuite) TearDownTest() {
	s.sr.Quit()
	s.secInterface.Deinitialize()
}

func (s *SectionRunnerSuite) TestRunSection() {
	s.secInterface.On("Set", (SectionID)(0), true).
		Return().
		Run(func(args mock.Arguments) {
			s.secInterface.On("Set", (SectionID)(0), false).Return()
		})

	s.sr.RunSection(&s.secs[0], 10*time.Nanosecond)
	s.secInterface.AssertAllCalled(s.T())
}

func (s *SectionRunnerSuite) TestSectionQueue() {
	s.secInterface.On("Set", (SectionID)(0), true).Return().
		Run(func(args mock.Arguments) {
			s.secInterface.On("Set", (SectionID)(0), false).Return()
			s.secInterface.On("Set", (SectionID)(1), true).Return().
				Run(func(args mock.Arguments) {
					s.secInterface.On("Set", (SectionID)(1), false).Return()
				})
		})

	s.sr.QueueSectionRun(&s.secs[0], 10*time.Nanosecond)
	s.sr.QueueSectionRun(&s.secs[1], 10*time.Nanosecond)
	time.Sleep(50 * time.Millisecond)
	s.secInterface.AssertAllCalled(s.T())
}

// func (s *SectionRunnerSuite) TestStateToJSON() {
// 	s.secInterface.SetupReturns(&s.secs[0])

// 	s.sr.QueueSectionRun(&s.secs[0], time.Minute)
// 	s.sr.QueueSectionRun(&s.secs[1], time.Minute)
// 	time.Sleep(10 * time.Millisecond)

// 	_, err := s.sr.State.ToJSON([]Section{&s.secs[0]})
// 	s.ass.Error(err, "should error because passed sections doesn't contian &s.secs[1]")
// }

func (s *SectionRunnerSuite) TestRunAsync() {
	s.secInterface.SetupReturns(&s.secs[0])
	s.secInterface.AssertNotRunning(s.T(), &s.secs[0])
	_, c := s.sr.RunSectionAsync(&s.secs[0], 50*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	s.secInterface.AssertRunning(s.T(), &s.secs[0])

	s.sr.State.Lock()
	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.Zero(s.sr.State.Queue.Len())
	s.sr.State.Unlock()

	<-c
	s.secInterface.AssertNotRunning(s.T(), &s.secs[0])

	s.sr.State.Lock()
	s.ass.Nil(s.sr.State.Current)
	s.ass.Zero(s.sr.State.Queue.Len())
	s.sr.State.Unlock()

	s.secInterface.AssertAllCalled(s.T())

}

func (s *SectionRunnerSuite) TestCancelSection() {
	s.secInterface.SetupAllReturns()

	s.sr.QueueSectionRun(&s.secs[0], time.Minute)
	s.sr.QueueSectionRun(&s.secs[1], time.Minute)

	time.Sleep(10 * time.Millisecond)
	s.sr.State.Lock()
	s.ass.Equal(s.sr.State.Queue.Len(), 1, "There should be 1 item in the queue")
	queue := s.sr.State.Queue.ToSlice()
	s.ass.Equal(1, queue[0].Sec.ID)
	s.ass.Equal(time.Minute, queue[0].Duration)
	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.Equal(time.Minute, s.sr.State.Current.Duration)
	s.sr.State.Unlock()
	s.secInterface.AssertNotRunning(s.T(), &s.secs[1])
	s.secInterface.AssertRunning(s.T(), &s.secs[0])

	s.sr.CancelSection(&s.secs[1])
	time.Sleep(10 * time.Millisecond)
	s.sr.State.Lock()
	s.ass.Zero(s.sr.State.Queue.Len(), "There should be 0 items in the queue")
	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.Equal(time.Minute, s.sr.State.Current.Duration)
	s.sr.State.Unlock()
	s.secInterface.AssertNotRunning(s.T(), &s.secs[1])
	s.secInterface.AssertRunning(s.T(), &s.secs[0])

	s.sr.QueueSectionRun(&s.secs[1], time.Minute)
	s.sr.CancelSection(&s.secs[0])
	time.Sleep(10 * time.Millisecond)
	s.sr.State.Lock()
	s.ass.Zero(s.sr.State.Queue.Len(), "There should be 0 items in the queue")
	s.ass.Equal(1, s.sr.State.Current.Sec.ID)
	s.ass.Equal(time.Minute, s.sr.State.Current.Duration)
	s.sr.State.Unlock()
	s.secInterface.AssertRunning(s.T(), &s.secs[1])
	s.secInterface.AssertNotRunning(s.T(), &s.secs[0])

	s.sr.CancelSection(&s.secs[1])
	time.Sleep(10 * time.Millisecond)

	s.secInterface.AssertAllCalled(s.T())

}

func (s *SectionRunnerSuite) TestCancelID() {
	s.secInterface.SetupAllReturns()

	id1 := s.sr.QueueSectionRun(&s.secs[0], time.Minute)
	id2 := s.sr.QueueSectionRun(&s.secs[1], time.Minute)

	time.Sleep(10 * time.Millisecond)
	s.sr.State.Lock()
	queue := s.sr.State.Queue.ToSlice()
	s.ass.Len(queue, 1, "There should be 1 item in the queue")
	s.ass.Equal(1, queue[0].Sec.ID)
	s.ass.Equal(60*time.Second, queue[0].Duration)
	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.Equal(60*time.Second, s.sr.State.Current.Duration)
	s.sr.State.Unlock()
	s.secInterface.AssertNotRunning(s.T(), &s.secs[1])
	s.secInterface.AssertRunning(s.T(), &s.secs[0])

	s.sr.CancelID(id2)
	time.Sleep(10 * time.Millisecond)
	s.sr.State.Lock()
	s.ass.Zero(s.sr.State.Queue.Len(), "There should be 0 items in the queue")
	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.Equal(60*time.Second, s.sr.State.Current.Duration)
	s.sr.State.Unlock()
	s.secInterface.AssertNotRunning(s.T(), &s.secs[1])
	s.secInterface.AssertRunning(s.T(), &s.secs[0])

	id2 = s.sr.QueueSectionRun(&s.secs[1], time.Minute)
	s.sr.CancelID(id1)
	time.Sleep(10 * time.Millisecond)
	s.sr.State.Lock()
	s.ass.Zero(s.sr.State.Queue.Len(), "There should be 0 items in the queue")
	s.ass.Equal(1, s.sr.State.Current.Sec.ID)
	s.ass.Equal(60*time.Second, s.sr.State.Current.Duration)
	s.sr.State.Unlock()
	s.secInterface.AssertRunning(s.T(), &s.secs[1])
	s.secInterface.AssertNotRunning(s.T(), &s.secs[0])

	s.sr.CancelID(id2)
	time.Sleep(10 * time.Millisecond)

	s.secInterface.AssertAllCalled(s.T())

}

func (s *SectionRunnerSuite) TestCancelAll() {
	s.secInterface.SetupAllReturns()

	s.sr.QueueSectionRun(&s.secs[0], time.Minute)
	s.sr.QueueSectionRun(&s.secs[1], time.Minute)

	time.Sleep(10 * time.Millisecond)
	s.secInterface.AssertNotRunning(s.T(), &s.secs[1])
	s.secInterface.AssertRunning(s.T(), &s.secs[0])

	s.sr.CancelAll()
	time.Sleep(10 * time.Millisecond)
	s.sr.State.Lock()
	s.ass.Zero(s.sr.State.Queue.Len(), "There should be 0 items in the queue")
	s.ass.Nil(s.sr.State.Current)
	s.sr.State.Unlock()
	s.secInterface.AssertNotRunning(s.T(), &s.secs[1])
	s.secInterface.AssertNotRunning(s.T(), &s.secs[0])
}

func (s *SectionRunnerSuite) TestPause() {
	s.secInterface.SetupAllReturns()

	id1 := s.sr.QueueSectionRun(&s.secs[0], time.Minute)
	time.Sleep(10 * time.Millisecond)
	s.ass.True(s.secs[0].GetState(s.secInterface), "Section should be running")

	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.Equal(time.Minute, s.sr.State.Current.Duration)
	s.ass.Equal(time.Minute, s.sr.State.Current.TotalDuration)
	s.ass.Nil(s.sr.State.Current.PauseTime)
	s.ass.Nil(s.sr.State.Current.UnpauseTime)

	s.sr.Pause()
	time.Sleep(10 * time.Millisecond)
	s.ass.True(s.sr.State.Paused, "SectionRunner should be paused")
	s.ass.False(s.secs[0].GetState(s.secInterface), "Section should not be running")

	s.ass.True(s.sr.State.Paused)
	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.Equal(time.Minute, s.sr.State.Current.TotalDuration)
	s.ass.InDelta(59.990*(float64)(time.Second), s.sr.State.Current.Duration, 10*(float64)(time.Millisecond))
	s.ass.NotNil(s.sr.State.Current.PauseTime)
	s.ass.Nil(s.sr.State.Current.UnpauseTime)

	s.sr.Pause() // double pause should change nothing
	time.Sleep(10 * time.Millisecond)
	s.ass.True(s.sr.State.Paused, "SectionRunner should be paused")
	s.ass.False(s.secs[0].GetState(s.secInterface), "Section should not be running")

	s.ass.True(s.sr.State.Paused)
	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.InDelta(59990*(float64)(time.Millisecond), s.sr.State.Current.Duration, 10*(float64)(time.Millisecond))
	s.ass.NotNil(s.sr.State.Current.PauseTime)

	s.sr.Unpause()
	time.Sleep(40 * time.Millisecond)
	s.ass.False(s.sr.State.Paused, "SectionRunner should not be paused")
	s.ass.True(s.secs[0].GetState(s.secInterface), "Section should be running")

	s.ass.False(s.sr.State.Paused)
	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.Equal(60*time.Second, s.sr.State.Current.TotalDuration)
	s.ass.InDelta(59990*(float64)(time.Millisecond), s.sr.State.Current.Duration, 10*(float64)(time.Millisecond))
	s.ass.Nil(s.sr.State.Current.PauseTime)
	s.ass.NotNil(s.sr.State.Current.UnpauseTime)

	s.sr.QueueSectionRun(&s.secs[1], 40*time.Millisecond)
	s.sr.Pause()
	time.Sleep(10 * time.Millisecond)

	s.ass.True(s.sr.State.Paused)
	s.ass.Equal(0, s.sr.State.Current.Sec.ID)
	s.ass.Equal(60*time.Second, s.sr.State.Current.TotalDuration)
	s.ass.InDelta(59950*(float64)(time.Millisecond), s.sr.State.Current.Duration, 10*(float64)(time.Millisecond))
	s.ass.NotNil(s.sr.State.Current.PauseTime)
	s.ass.NotNil(s.sr.State.Current.UnpauseTime)

	s.sr.CancelID(id1)
	time.Sleep(10 * time.Millisecond)
	s.ass.True(s.sr.State.Paused, "SectionRunner should be paused")
	s.ass.False(s.secs[0].GetState(s.secInterface), "Section should not be running")
	s.ass.False(s.secs[1].GetState(s.secInterface), "Section should not be running")

	s.ass.True(s.sr.State.Paused)
	s.ass.Equal(1, s.sr.State.Current.Sec.ID)
	s.ass.Equal(40*time.Millisecond, s.sr.State.Current.Duration)
	s.ass.NotNil(s.sr.State.Current.PauseTime)

	s.sr.Unpause()
	time.Sleep(20 * time.Millisecond)
	s.ass.True(s.secs[1].GetState(s.secInterface), "Section should be running")

	s.ass.False(s.sr.State.Paused)
	s.ass.Equal(1, s.sr.State.Current.Sec.ID)
	s.ass.Equal(40*time.Millisecond, s.sr.State.Current.Duration)
	s.ass.Nil(s.sr.State.Current.PauseTime)

	s.sr.Pause()
	time.Sleep(10 * time.Millisecond)
	s.ass.False(s.secs[1].GetState(s.secInterface), "Section should not be running")

	s.ass.True(s.sr.State.Paused)
	s.ass.Equal(1, s.sr.State.Current.Sec.ID)
	s.ass.Equal(40*time.Millisecond, s.sr.State.Current.TotalDuration)
	s.ass.InDelta(10*(float64)(time.Millisecond), s.sr.State.Current.Duration, 10*(float64)(time.Millisecond))
	s.ass.NotNil(s.sr.State.Current.PauseTime)

	s.sr.Unpause()
	time.Sleep(30 * time.Millisecond)
	// It should have started for 20 ms, then paused for 10 ms, then run again for 30 ms.
	// So 20ms + 30ms > 40ms, should be done
	s.ass.False(s.secs[1].GetState(s.secInterface), "Section should not be running")

	s.ass.False(s.sr.State.Paused)
	s.ass.Nil(s.sr.State.Current)

	s.secInterface.AssertAllCalled(s.T())
}

func TestSectionRunner(t *testing.T) {
	suite.Run(t, new(SectionRunnerSuite))
}
