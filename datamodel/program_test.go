package datamodel

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"

	"git.amikhalev.com/amikhalev/grinklers/logic"
	. "git.amikhalev.com/amikhalev/grinklers/sched"
	"git.amikhalev.com/amikhalev/grinklers/util"
	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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

func TestProgItem_JSON(t *testing.T) {
	ass := assert.New(t)
	req := require.New(t)
	sections := []logic.Section{
		logic.NewMockSection(0, "mock 1", t), logic.NewMockSection(1, "mock 2", t),
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

	pij2, err := ProgItemToJSON(pi)
	req.NoError(err)
	ass.Equal(1, pij2.Section)
	ass.Equal(60.0, pij2.Duration)
}

func TestProgram_JSON(t *testing.T) {
	ass := assert.New(t)
	req := require.New(t)
	sections := []logic.Section{
		&logic.RpioSection{}, &logic.RpioSection{},
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
	  	"schedule": {
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
	ass.Equal(logic.ProgItem{Sec: sections[0], Duration: 1*time.Hour + 2*time.Minute + 3*time.Second}, prog.Sequence[0])
	ass.Equal(logic.ProgItem{Sec: sections[1], Duration: 24 * time.Millisecond}, prog.Sequence[1])
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

	progJSON.Sequence = ProgSequenceJSON{ProgItemJSON{10, 0}}
	_, err = progJSON.ToProgram(sections)
	ass.Error(err)

	progJSON.Sequence = nil
	_, err = progJSON.ToProgram(sections)
	ass.NoError(err) // nil is a valid slice

	progJSON.Name = nil
	_, err = progJSON.ToProgram(sections)
	ass.Error(err)

	progJSON, err = ProgramToJSON(prog)
	req.NoError(err)

	_, err = json.Marshal(&progJSON)
	req.NoError(err)
}

func TestPrograms_JSON(t *testing.T) {
	ass := assert.New(t)
	req := require.New(t)
	sections := []logic.Section{
		&logic.RpioSection{}, &logic.RpioSection{},
	}

	str := `[
	{
		"name": "p1", "sequence": []
	}, {
		"name": "p2", "sequence": [{"section": 0, "duration": 60.0}],
		"schedule": {},
		"enabled": true
	}
	]`

	var psj ProgramsJSON
	err := json.Unmarshal([]byte(str), &psj)
	req.NoError(err)

	req.Len(psj, 2)

	ass.Equal("p1", *psj[0].Name)
	ass.Len(psj[0].Sequence, 0)

	ass.Equal("p2", *psj[1].Name)
	req.Len(psj[1].Sequence, 1)
	ass.Equal(0, (psj[1].Sequence)[0].Section)
	ass.Equal(60.0, (psj[1].Sequence)[0].Duration)
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

	(psj[1].Sequence)[0].Section = 3
	_, err = psj.ToPrograms(sections)
	ass.Error(err)

	psj, err = ProgramsToJSON(ps, sections)
	req.NoError(err)

	req.Len(psj, 2)

	ass.Equal("p1", *psj[0].Name)
	ass.Len(psj[0].Sequence, 0)

	ass.Equal("p2", *psj[1].Name)
	req.Len(psj[1].Sequence, 1)
	ass.Equal(0, (psj[1].Sequence)[0].Section)
	ass.Equal(60.0, (psj[1].Sequence)[0].Duration)
	ass.Equal(true, *psj[1].Enabled)
}

type ProgramSuite struct {
	ass       *assert.Assertions
	req       *require.Assertions
	sec1      *logic.MockSection
	sec2      *logic.MockSection
	secRunner *logic.SectionRunner
	waitGroup *sync.WaitGroup
	program   *logic.Program
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
	s.sec1 = logic.NewMockSection(0, "sec1", s.T())
	s.sec2 = logic.NewMockSection(1, "sec2", s.T())
	s.secRunner = logic.NewSectionRunner()
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
func (s *ProgramSuite) TestProgram_Update() {
	ass, req, secRunner := s.ass, s.req, s.secRunner
	s.SetupSecs()

	prog := logic.NewProgram("test_update", []logic.ProgItem{
		{Sec: s.sec1, Duration: 25 * time.Millisecond},
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
	name := "test2"
	running := true
	progJSON := NewProgramJSON(&name, newSeq, &newSched, &running)
	err := progJSON.Update(prog, []logic.Section{s.sec1, s.sec2})
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
	progJSON = NewProgramJSON(nil, newSeq, nil, nil)
	err = progJSON.Update(prog, []logic.Section{s.sec1, s.sec2})
	ass.Error(err)
}

func TestProgramSuite(t *testing.T) {
	suite.Run(t, new(ProgramSuite))
}
