package grinklers

import (
	"encoding/json"
	"testing"
	"time"

	"io/ioutil"
	"os"
	"sync"

	"github.com/Sirupsen/logrus"
	. "github.com/amikhalev/grinklers/sched"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func makeSchedule() Schedule {
	runTime := time.Now().Add(10 * time.Millisecond)
	return Schedule{
		Times: []TimeOfDay{
			TimeOfDay{Hour: runTime.Hour(), Minute: runTime.Minute(), Second: runTime.Second(), Millisecond: runTime.Nanosecond() / 1000000},
		},
		Weekdays: EveryDay,
	}
}

func TestProgItem_JSON(t *testing.T) {
	ass := assert.New(t)
	req := require.New(t)
	sections := []Section{
		&RpioSection{}, &RpioSection{},
	}

	str := `{
		"section": 1,
		"duration": 60.0
	}`
	var pij ProgItemJSON
	err := json.Unmarshal([]byte(str), &pij)
	req.NoError(err)
	ass.Equal(1, pij.Section)
	ass.Equal(60.0, pij.Duration)

	pi, err := pij.ToProgItem(sections)
	req.NoError(err)
	ass.Equal(float64(1.0), pi.Duration.Minutes())
	ass.Equal(sections[1], pi.Sec)

	pij.Duration = 60.0
	pij.Section = 5 // out of range
	_, err = pij.ToProgItem(sections)
	ass.Error(err)

	pij2, err := pi.ToJSON(sections)
	req.NoError(err)
	ass.Equal(1, pij2.Section)
	ass.Equal(60.0, pij2.Duration)

	pi.Sec = &RpioSection{} // not in sections array
	_, err = pi.ToJSON(sections)
	ass.Error(err)
}

func TestProgram_JSON(t *testing.T) {
	ass := assert.New(t)
	req := require.New(t)
	sections := []Section{
		&RpioSection{}, &RpioSection{},
	}

	str := `{
		"name": "test 1234",
	 	"sequence": [{
	 		"section": 0,
	 		"duration": 3723.0
	 	}, {
	 		"section": 1,
	 		"duration": 0.024
	 	}],
	  	"sched": {
	  		"times": [{
	  			"hour": 1, "minute": 2
	  		}],
	  		"weekdays": [1, 3, 5]
	  	},
	   	"enabled": true
	   }`
	var progJSON ProgramJSON
	err := json.Unmarshal([]byte(str), &progJSON)
	req.NoError(err)
	prog, err := progJSON.ToProgram(sections)
	req.NoError(err)
	ass.Equal("test 1234", prog.Name)
	ass.Equal(true, prog.Enabled)
	ass.Equal(ProgItem{sections[0], 1*time.Hour + 2*time.Minute + 3*time.Second}, prog.Sequence[0])
	ass.Equal(ProgItem{sections[1], 24 * time.Millisecond}, prog.Sequence[1])
	ass.Equal(TimeOfDay{Hour: 1, Minute: 2, Second: 0, Millisecond: 0}, prog.Sched.Times[0])
	ass.Contains(prog.Sched.Weekdays, time.Monday)
	ass.Contains(prog.Sched.Weekdays, time.Wednesday)
	ass.Contains(prog.Sched.Weekdays, time.Friday)

	progJSON.Enabled = nil
	_, err = progJSON.ToProgram(sections)
	ass.NoError(err)

	progJSON.Sched = nil
	_, err = progJSON.ToProgram(sections)
	ass.NoError(err)

	*(progJSON.Sequence) = ProgSequenceJSON{ProgItemJSON{10, 0}}
	_, err = progJSON.ToProgram(sections)
	ass.Error(err)

	progJSON.Sequence = nil
	_, err = progJSON.ToProgram(sections)
	ass.Error(err)

	progJSON.Name = nil
	_, err = progJSON.ToProgram(sections)
	ass.Error(err)

	progJSON, err = prog.ToJSON(sections)
	req.NoError(err)

	prog.Sequence[0].Sec = &RpioSection{}
	_, err = prog.ToJSON(sections)
	ass.Error(err)

	_, err = json.Marshal(&progJSON)
	req.NoError(err)
}

func TestPrograms_JSON(t *testing.T) {
	ass := assert.New(t)
	req := require.New(t)
	sections := []Section{
		&RpioSection{}, &RpioSection{},
	}

	str := `[
	{
		"name": "p1", "sequence": []
	}, {
		"name": "p2", "sequence": [{"section": 0, "duration": 60.0}],
		"sched": {},
		"enabled": true
	}
	]`

	var psj ProgramsJSON
	err := json.Unmarshal([]byte(str), &psj)
	req.NoError(err)

	req.Len(psj, 2)

	ass.Equal("p1", *psj[0].Name)
	ass.Len(*psj[0].Sequence, 0)

	ass.Equal("p2", *psj[1].Name)
	req.Len(*psj[1].Sequence, 1)
	ass.Equal(0, (*psj[1].Sequence)[0].Section)
	ass.Equal(60.0, (*psj[1].Sequence)[0].Duration)
	ass.Equal(true, *psj[1].Enabled)

	ps, err := psj.ToPrograms(sections)
	req.NoError(err)

	req.Len(ps, 2)
	ass.Equal("p1", ps[0].Name)
	ass.Len(ps[0].Sequence, 0)
	ass.Equal(false, ps[0].Enabled)

	ass.Equal("p2", ps[1].Name)
	req.Len(ps[1].Sequence, 1)
	ass.Equal(sections[0], ps[1].Sequence[0].Sec)
	ass.Equal("1m0s", ps[1].Sequence[0].Duration.String())
	ass.Equal(true, ps[1].Enabled)

	(*psj[1].Sequence)[0].Section = 3
	_, err = psj.ToPrograms(sections)
	ass.Error(err)

	psj, err = ProgramsToJSON(ps, sections)
	req.NoError(err)

	req.Len(psj, 2)

	ass.Equal("p1", *psj[0].Name)
	ass.Len(*psj[0].Sequence, 0)

	ass.Equal("p2", *psj[1].Name)
	req.Len(*psj[1].Sequence, 1)
	ass.Equal(0, (*psj[1].Sequence)[0].Section)
	ass.Equal(60.0, (*psj[1].Sequence)[0].Duration)
	ass.Equal(true, *psj[1].Enabled)

	ps[1].Sequence[0].Sec = &RpioSection{}
	_, err = ProgramsToJSON(ps, sections)
	ass.Error(err)
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
	Logger.Out = ioutil.Discard
	/*Logger.Out =*/ _ = os.Stdout
	Logger.Level = logrus.DebugLevel
	Logger.Warn("ayyyo")
	s.sec1 = newMockSection("sec1", s.T())
	s.sec2 = newMockSection("sec2", s.T())
	s.secRunner = NewSectionRunner()
	s.secRunner.Start(s.waitGroup)
}

func (s *ProgramSuite) TearDownTest() {
	s.secRunner.Quit()
	s.waitGroup.Wait()
}

func (s *ProgramSuite) TestProgram_Run() {
	ass, secRunner := s.ass, s.secRunner

	onUpdate := make(chan ProgUpdate, 10)

	prog := NewProgram("test_run", []ProgItem{
		ProgItem{s.sec1, 10 * time.Millisecond},
		ProgItem{s.sec2, 10 * time.Millisecond},
	}, Schedule{}, false)
	prog.OnUpdate = onUpdate
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
	ass.Equal(prog, p.Prog)
	ass.Equal(pupdateRunning, p.Type)
	ass.Equal(true, prog.Running())

	p = <-onUpdate
	ass.Equal(prog, p.Prog)
	ass.Equal(pupdateRunning, p.Type)
	ass.Equal(false, prog.Running())

	s.sec1.AssertExpectations(s.T())
	s.sec2.AssertExpectations(s.T())

	prog.Quit()
}

func (s *ProgramSuite) SetupSecs() {
	s.sec1.SetupReturns()
	s.sec2.SetupReturns()
}

func (s *ProgramSuite) TestProgram_Schedule() {
	ass, secRunner := s.ass, s.secRunner
	s.SetupSecs()

	prog := NewProgram("test_schedule", []ProgItem{
		ProgItem{s.sec1, 25 * time.Millisecond},
		ProgItem{s.sec2, 25 * time.Millisecond},
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
		ProgItem{s.sec1, 25 * time.Millisecond},
		ProgItem{s.sec2, 25 * time.Millisecond},
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
		ProgItem{s.sec1, 25 * time.Millisecond},
		ProgItem{s.sec2, 25 * time.Millisecond},
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
		ProgItem{s.sec1, 25 * time.Millisecond},
		ProgItem{s.sec2, 25 * time.Millisecond},
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

func (s *ProgramSuite) TestProgram_Update() {
	ass, req, secRunner := s.ass, s.req, s.secRunner
	s.SetupSecs()

	prog := NewProgram("test_update", []ProgItem{
		ProgItem{s.sec1, 25 * time.Millisecond},
	}, makeSchedule(), false)

	prog.Start(secRunner, nil)

	time.Sleep(20 * time.Millisecond)
	ass.Equal(false, prog.Running())
	time.Sleep(20 * time.Millisecond)
	ass.Equal(false, prog.Running())

	newSeq := ProgSequenceJSON{
		ProgItemJSON{0, 0.025},
		ProgItemJSON{1, 0.025},
	}
	newSched := makeSchedule()
	err := prog.Update(NewProgramJSON("test2", newSeq, &newSched, true), []Section{s.sec1, s.sec2})
	req.NoError(err)

	ass.Equal("test2", prog.Name)
	req.Len(prog.Sequence, 2)
	ass.Equal(true, prog.Enabled)

	time.Sleep(20 * time.Millisecond)
	ass.Equal(true, prog.Running())
	time.Sleep(60 * time.Millisecond)
	ass.Equal(false, prog.Running())

	prog.Quit()

	s.sec1.AssertNumberOfCalls(s.T(), "SetState", 2)
	s.sec2.AssertNumberOfCalls(s.T(), "SetState", 2)

	newSeq[0].Section = 3
	err = prog.Update(ProgramJSON{nil, &newSeq, nil, nil}, []Section{s.sec1, s.sec2})
	ass.Error(err)
}

func TestProgramSuite(t *testing.T) {
	suite.Run(t, new(ProgramSuite))
}
