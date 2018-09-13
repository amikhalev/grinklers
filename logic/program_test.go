package logic

import (
	"testing"
	"time"

	"io/ioutil"
	"os"
	"sync"

	. "git.amikhalev.com/amikhalev/grinklers/sched"
	"git.amikhalev.com/amikhalev/grinklers/util"
	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func makeSchedule() Schedule {
	runTime := time.Now().Add(10 * time.Millisecond)
	return Schedule{
		Times: []TimeOfDay{
			{Hour: runTime.Hour(), Minute: runTime.Minute(), Second: runTime.Second(), Millisecond: runTime.Nanosecond() / 1000000},
		},
		Weekdays: EveryDay,
	}
}

type ProgramSuite struct {
	ass       *assert.Assertions
	req       *require.Assertions
	sec1      *MockSection
	sec2      *MockSection
	secRunner *SectionRunner
	waitGroup *sync.WaitGroup
	program   *Program
	suite.Suite
}

func (s *ProgramSuite) SetupSuite() {
	s.ass = assert.New(s.T())
	s.req = require.New(s.T())
	s.waitGroup = &sync.WaitGroup{}
}

func (s *ProgramSuite) SetupTest() {
	util.Logger.Out = ioutil.Discard
	/*Logger.Out =*/ _ = os.Stdout
	util.Logger.Level = logrus.DebugLevel
	util.Logger.Warn("ayyyo")
	s.sec1 = NewMockSection(0, "sec1", s.T())
	s.sec2 = NewMockSection(1, "sec2", s.T())
	s.secRunner = NewSectionRunner()
	s.secRunner.Start(s.waitGroup)
}

func (s *ProgramSuite) SetupSecs() {
	s.sec1.SetupReturns()
	s.sec2.SetupReturns()
}

func (s *ProgramSuite) TearDownTest() {
	s.secRunner.Quit()
	s.waitGroup.Wait()
}

func (s *ProgramSuite) TestProgram_Run() {
	ass, secRunner := s.ass, s.secRunner

	onUpdate := make(chan ProgUpdate, 10)

	prog := NewProgram("test_run", []ProgItem{
		{s.sec1, 10 * time.Millisecond},
		{s.sec2, 10 * time.Millisecond},
	}, Schedule{}, false)
	prog.UpdateChan = onUpdate
	prog.Start(secRunner, s.waitGroup)

	s.sec1.On("SetState", true).Return().
		Run(func(_ mock.Arguments) {
			s.sec1.On("SetState", false).Return().
				Run(func(_ mock.Arguments) {
					s.sec2.On("SetState", true).Return().
						Run(func(_ mock.Arguments) {
							s.sec2.On("SetState", false).Return()
						})
				})
		})

	prog.Run()

	p := <-onUpdate
	ass.Equal(&prog, &p.Prog)
	ass.Equal(ProgUpdateRunning, p.Type)
	ass.Equal(true, prog.Running())

	p = <-onUpdate
	ass.Equal(&prog, &p.Prog)
	ass.Equal(ProgUpdateRunning, p.Type)
	ass.Equal(false, prog.Running())

	s.sec1.AssertExpectations(s.T())
	s.sec2.AssertExpectations(s.T())

	prog.Quit()
}

func (s *ProgramSuite) TestProgram_Schedule() {
	ass, secRunner := s.ass, s.secRunner
	s.SetupSecs()

	prog := NewProgram("test_schedule", []ProgItem{
		{s.sec1, 25 * time.Millisecond},
		{s.sec2, 25 * time.Millisecond},
	}, makeSchedule(), true)
	prog.Start(secRunner, s.waitGroup)

	time.Sleep(50 * time.Millisecond)
	ass.Equal(true, prog.Running())
	time.Sleep(50 * time.Millisecond)
	ass.Equal(false, prog.Running())

	prog.Quit()
}

func (s *ProgramSuite) TestProgram_OnUpdate() {
	ass, secRunner := s.ass, s.secRunner
	s.SetupSecs()

	prog := NewProgram("test_onupdate", []ProgItem{
		{s.sec1, 25 * time.Millisecond},
		{s.sec2, 25 * time.Millisecond},
	}, makeSchedule(), true)
	prog.Start(secRunner, s.waitGroup)

	time.Sleep(50 * time.Millisecond)
	ass.Equal(true, prog.Running())
	time.Sleep(50 * time.Millisecond)
	ass.Equal(false, prog.Running())

	prog.Quit()
}

func (s *ProgramSuite) TestProgram_DoubleRun() {
	ass, secRunner := s.ass, s.secRunner
	s.SetupSecs()

	prog := NewProgram("test_doublerun", []ProgItem{
		{s.sec1, 25 * time.Millisecond},
		{s.sec2, 25 * time.Millisecond},
	}, Schedule{}, false)
	prog.Start(secRunner, s.waitGroup)

	prog.Run()
	prog.Run()
	time.Sleep(15 * time.Millisecond)
	ass.Equal(true, prog.Running())
	time.Sleep(60 * time.Millisecond)
	ass.Equal(false, prog.Running())

	prog.Quit()

	s.sec1.AssertNumberOfCalls(s.T(), "SetState", 2)
	s.sec1.AssertNumberOfCalls(s.T(), "SetState", 2)
}

func (s *ProgramSuite) TestProgram_Cancel() {
	ass, secRunner := s.ass, s.secRunner
	s.SetupSecs()

	prog := NewProgram("test_cancel", []ProgItem{
		{s.sec1, 25 * time.Millisecond},
		{s.sec2, 25 * time.Millisecond},
	}, Schedule{}, false)
	prog.Start(secRunner, nil)

	prog.Run()
	time.Sleep(15 * time.Millisecond)
	ass.Equal(true, prog.Running())
	prog.Cancel()
	time.Sleep(15 * time.Millisecond)
	ass.Equal(false, prog.Running())
	time.Sleep(50 * time.Millisecond)

	prog.Quit()

	s.sec1.AssertNumberOfCalls(s.T(), "SetState", 2)
	s.sec2.AssertNumberOfCalls(s.T(), "SetState", 0)
}

func (s *ProgramSuite) TestProgram_SectionCancelled() {
	ass, secRunner := s.ass, s.secRunner
	s.SetupSecs()

	prog := NewProgram("test_section_cancelled", []ProgItem{
		{s.sec1, 25 * time.Millisecond},
		{s.sec2, 25 * time.Millisecond},
	}, Schedule{}, false)
	prog.Start(secRunner, nil)

	prog.Run()
	time.Sleep(15 * time.Millisecond)
	ass.Equal(true, prog.Running())
	s.secRunner.CancelSection(s.sec2)
	time.Sleep(15 * time.Millisecond)
	secRunner.State.Lock()
	ass.Nil(secRunner.State.Current)
	ass.Equal(secRunner.State.Queue.Len(), 0)
	secRunner.State.Unlock()
	ass.Equal(false, prog.Running())

	prog.Quit()

	s.sec1.AssertNumberOfCalls(s.T(), "SetState", 2)
	s.sec2.AssertNumberOfCalls(s.T(), "SetState", 0)
}

func TestProgramSuite(t *testing.T) {
	suite.Run(t, new(ProgramSuite))
}
