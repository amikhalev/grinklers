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
	ass          *assert.Assertions
	req          *require.Assertions
	sections     []Section
	secInterface *MockSectionInterface
	secRunner    *SectionRunner
	waitGroup    *sync.WaitGroup
	program      *Program
	suite.Suite
}

func (s *ProgramSuite) SetupSuite() {
	util.Logger.Out = ioutil.Discard
	/*Logger.Out =*/ _ = os.Stdout
	util.Logger.Level = logrus.DebugLevel
	s.sections = []Section{
		Section{ID: 0, Name: "mock 0", InterfaceID: 0},
		Section{ID: 1, Name: "mock 1", InterfaceID: 1},
	}

	s.ass = assert.New(s.T())
	s.req = require.New(s.T())
	s.waitGroup = &sync.WaitGroup{}
	s.secInterface = NewMockSectionInterface(2)
}

func (s *ProgramSuite) SetupTest() {
	s.secInterface.Initialize()
	s.secRunner = NewSectionRunner(s.secInterface)
	s.secRunner.Start(s.waitGroup)
}

func (s *ProgramSuite) TearDownTest() {
	s.secRunner.Quit()
	s.waitGroup.Wait()
	s.secInterface.Deinitialize()
}

func (s *ProgramSuite) TestProgram_Run() {
	ass, secRunner := s.ass, s.secRunner

	onUpdate := make(chan ProgUpdate, 10)

	prog := NewProgram("test_run", []ProgItem{
		{&s.sections[0], 10 * time.Millisecond},
		{&s.sections[1], 10 * time.Millisecond},
	}, Schedule{}, false)
	prog.SetUpdateChan(onUpdate)
	prog.Start(secRunner, s.waitGroup)

	s.secInterface.On("Set", (SectionID)(0), true).Return().
		Run(func(_ mock.Arguments) {
			s.secInterface.On("Set", (SectionID)(0), false).Return().
				Run(func(_ mock.Arguments) {
					s.secInterface.On("Set", (SectionID)(1), true).Return().
						Run(func(_ mock.Arguments) {
							s.secInterface.On("Set", (SectionID)(1), false).Return()
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

	s.secInterface.AssertExpectations(s.T())

	prog.Quit()
}

func (s *ProgramSuite) TestProgram_Schedule() {
	ass, secRunner := s.ass, s.secRunner
	s.secInterface.SetupAllReturns()

	prog := NewProgram("test_schedule", []ProgItem{
		{&s.sections[0], 25 * time.Millisecond},
		{&s.sections[1], 25 * time.Millisecond},
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
	s.secInterface.SetupAllReturns()

	prog := NewProgram("test_onupdate", []ProgItem{
		{&s.sections[0], 25 * time.Millisecond},
		{&s.sections[1], 25 * time.Millisecond},
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
	s.secInterface.SetupAllReturns()

	prog := NewProgram("test_doublerun", []ProgItem{
		{&s.sections[0], 25 * time.Millisecond},
		{&s.sections[1], 25 * time.Millisecond},
	}, Schedule{}, false)
	prog.Start(secRunner, s.waitGroup)

	prog.Run()
	prog.Run()
	time.Sleep(15 * time.Millisecond)
	ass.Equal(true, prog.Running())
	time.Sleep(60 * time.Millisecond)
	ass.Equal(false, prog.Running())

	prog.Quit()

	s.secInterface.AssertNumberOfCalls(s.T(), "Set", 4)
}

func (s *ProgramSuite) TestProgram_Cancel() {
	ass, secRunner := s.ass, s.secRunner
	s.secInterface.SetupAllReturns()

	prog := NewProgram("test_cancel", []ProgItem{
		{&s.sections[0], 25 * time.Millisecond},
		{&s.sections[1], 25 * time.Millisecond},
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

	s.secInterface.AssertNumberOfCalls(s.T(), "Set", 2)
}

func (s *ProgramSuite) TestProgram_SectionCancelled() {
	ass, secRunner := s.ass, s.secRunner
	s.secInterface.SetupAllReturns()

	prog := NewProgram("test_section_cancelled", []ProgItem{
		{&s.sections[0], 25 * time.Millisecond},
		{&s.sections[1], 25 * time.Millisecond},
	}, Schedule{}, false)
	prog.Start(secRunner, nil)

	prog.Run()
	time.Sleep(15 * time.Millisecond)
	ass.Equal(true, prog.Running())
	s.secRunner.CancelSection(&s.sections[1])
	time.Sleep(15 * time.Millisecond)
	secRunner.State.Lock()
	ass.Nil(secRunner.State.Current)
	ass.Equal(secRunner.State.Queue.Len(), 0)
	secRunner.State.Unlock()
	ass.Equal(false, prog.Running())

	prog.Quit()

	s.secInterface.AssertNumberOfCalls(s.T(), "Set", 2)
}

func TestProgramSuite(t *testing.T) {
	suite.Run(t, new(ProgramSuite))
}
