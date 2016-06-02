package grinklers

import (
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	. "github.com/amikhalev/grinklers/sched"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func makeSchedule() Schedule {
	runTime := time.Now().Add(10 * time.Millisecond)
	return Schedule{
		Times: []TimeOfDay{
			TimeOfDay{runTime.Hour(), runTime.Minute(), runTime.Second(), runTime.Nanosecond() / 1000000},
		},
		Weekdays: EveryDay,
	}
}

func TestProgItem_JSON(t *testing.T) {
	ass := assert.New(t)
	Logger.Out = ioutil.Discard

	sections := []Section {
		&RpioSection{}, &RpioSection{},
	}

	str := `{
		"section": 1,
		"duration": "1m"
	}`
	var pij ProgItemJSON
	err := json.Unmarshal([]byte(str), &pij)
	require.NoError(t, err)
	ass.Equal(1, pij.Section)
	ass.Equal("1m", pij.Duration)

	pi, err := pij.ToProgItem(sections)
	require.NoError(t, err)
	ass.Equal(float64(1.0), pi.Duration.Minutes())
	ass.Equal(sections[1], pi.Sec)

	pij.Duration = "should not parse as a valid duration"
	_, err = pij.ToProgItem(sections)
	ass.Error(err)

	pij.Duration = "1m"
	pij.Section = 5 // out of range
	_, err = pij.ToProgItem(sections)
	ass.Error(err)

	pij2, err := pi.ToJSON(sections)
	require.NoError(t, err)
	ass.Equal(1, pij2.Section)
	ass.Equal("1m0s", pij2.Duration)

	pi.Sec = &RpioSection{} // not in sections array
	_, err = pi.ToJSON(sections)
	ass.Error(err)
}

func TestProgram_JSON(t *testing.T) {
	Logger.Out = ioutil.Discard
	ass := assert.New(t)

	sections := []Section{
		&RpioSection{}, &RpioSection{},
	}

	str := `{
		"name": "test 1234",
	 	"sequence": [{
	 		"section": 0,
	 		"duration": "1h2m3s"
	 	}, {
	 		"section": 1,
	 		"duration": "24ms"
	 	}],
	  	"sched": {
	  		"times": [{
	  			"hour": 1, "minute": 2
	  		}],
	  		"weekdays": [1, 3, 5]
	  	},
	   	"enabled": true
	   }`
	var progJson ProgramJSON
	err := json.Unmarshal([]byte(str), &progJson)
	require.NoError(t, err)
	prog, err := progJson.ToProgram(sections)
	require.NoError(t, err)
	ass.Equal("test 1234", prog.Name)
	ass.Equal(true, prog.Enabled)
	ass.Equal(ProgItem{sections[0], 1*time.Hour + 2*time.Minute + 3*time.Second}, prog.Sequence[0])
	ass.Equal(ProgItem{sections[1], 24 * time.Millisecond}, prog.Sequence[1])
	ass.Equal(TimeOfDay{1, 2, 0, 0}, prog.Sched.Times[0])
	ass.Contains(prog.Sched.Weekdays, time.Monday)
	ass.Contains(prog.Sched.Weekdays, time.Wednesday)
	ass.Contains(prog.Sched.Weekdays, time.Friday)

	progJson.Enabled = nil
	_, err = progJson.ToProgram(sections)
	ass.NoError(err)

	progJson.Sched = nil
	_, err = progJson.ToProgram(sections)
	ass.NoError(err)

	*(progJson.Sequence) = ProgSequenceJSON{ProgItemJSON{3, ""}}
	_, err = progJson.ToProgram(sections)
	ass.Error(err)

	progJson.Sequence = nil
	_, err = progJson.ToProgram(sections)
	ass.Error(err)

	progJson.Name = nil
	_, err = progJson.ToProgram(sections)
	ass.Error(err)

	progJson, err = prog.ToJSON(sections)
	require.NoError(t, err)

	prog.Sequence[0].Sec = &RpioSection{}
	_, err = prog.ToJSON(sections)
	ass.Error(err)

	_, err = json.Marshal(&progJson)
	require.NoError(t, err)
}

func TestProgram_Run(t *testing.T) {
	sec1 := newMockSection("sec1")
	sec2 := newMockSection("sec2")

	secRunner := NewSectionRunner()

	onUpdate := make(chan *Program, 10)

	prog := NewProgram("test", []ProgItem{
		ProgItem{sec1, 10 * time.Millisecond},
		ProgItem{sec2, 10 * time.Millisecond},
	}, Schedule{}, false)
	prog.OnUpdate = onUpdate
	prog.Start(secRunner)

	sec1.On("SetState", true).Return().
		Run(func(_ mock.Arguments) {
			sec1.On("SetState", false).Return().
				Run(func(_ mock.Arguments) {
					sec2.On("SetState", true).Return().
						Run(func(_ mock.Arguments) {
							sec2.On("SetState", false).Return()
						})
				})
		})

	prog.Run()

	p := <-onUpdate
	assert.Equal(t, prog, p)
	assert.Equal(t, true, prog.running)

	p = <-onUpdate
	assert.Equal(t, prog, p)
	assert.Equal(t, false, prog.running)

	sec1.AssertExpectations(t)
	sec2.AssertExpectations(t)

	prog.Quit()
}

func TestProgram_Schedule(t *testing.T) {
	//logger.SetHandler(log15.StdoutHandler)
	sec1 := newMockSection("sec1")
	sec2 := newMockSection("sec2")

	sec1.On("SetState", true).Return()
	sec1.On("SetState", false).Return()
	sec2.On("SetState", true).Return()
	sec2.On("SetState", false).Return()

	secRunner := NewSectionRunner()

	prog := NewProgram("test", []ProgItem{
		ProgItem{sec1, 25 * time.Millisecond},
		ProgItem{sec2, 25 * time.Millisecond},
	}, makeSchedule(), true)
	prog.Start(secRunner)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, true, prog.running)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, false, prog.running)

	prog.Quit()
}

func TestProgram_OnUpdate(t *testing.T) {
	sec1 := newMockSection("sec1")
	sec2 := newMockSection("sec2")

	sec1.On("SetState", true).Return()
	sec1.On("SetState", false).Return()
	sec2.On("SetState", true).Return()
	sec2.On("SetState", false).Return()

	secRunner := NewSectionRunner()

	prog := NewProgram("test", []ProgItem{
		ProgItem{sec1, 25 * time.Millisecond},
		ProgItem{sec2, 25 * time.Millisecond},
	}, makeSchedule(), true)
	prog.Start(secRunner)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, true, prog.running)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, false, prog.running)

	prog.Quit()
}

func TestProgram_DoubleRun(t *testing.T) {
	sec1 := newMockSection("sec1")
	sec2 := newMockSection("sec2")

	sec1.On("SetState", true).Return()
	sec1.On("SetState", false).Return()
	sec2.On("SetState", true).Return()
	sec2.On("SetState", false).Return()

	secRunner := NewSectionRunner()

	prog := NewProgram("test", []ProgItem{
		ProgItem{sec1, 25 * time.Millisecond},
		ProgItem{sec2, 25 * time.Millisecond},
	}, Schedule{}, false)
	prog.Start(secRunner)

	prog.Run()
	prog.Run()
	time.Sleep(15 * time.Millisecond)
	assert.Equal(t, true, prog.running)
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, false, prog.running)

	prog.Quit()

	sec1.AssertNumberOfCalls(t, "SetState", 2)
	sec2.AssertNumberOfCalls(t, "SetState", 2)
}

func TestProgram_Cancel(t *testing.T) {
	sec1 := newMockSection("sec1")
	sec2 := newMockSection("sec2")

	sec1.On("SetState", true).Return()
	sec1.On("SetState", false).Return()
	sec2.On("SetState", true).Return()
	sec2.On("SetState", false).Return()

	secRunner := NewSectionRunner()

	prog := NewProgram("test", []ProgItem{
		ProgItem{sec1, 25 * time.Millisecond},
		ProgItem{sec2, 25 * time.Millisecond},
	}, Schedule{}, false)
	prog.Start(secRunner)

	prog.Run()
	time.Sleep(15 * time.Millisecond)
	assert.Equal(t, true, prog.running)
	prog.Cancel()
	time.Sleep(15 * time.Millisecond)
	assert.Equal(t, false, prog.running)
	time.Sleep(50 * time.Millisecond)

	prog.Quit()

	sec1.AssertNumberOfCalls(t, "SetState", 2)
	sec2.AssertNumberOfCalls(t, "SetState", 0)
}

func TestProgram_Update(t *testing.T) {
	ass := assert.New(t)
	sec1 := newMockSection("sec1")
	sec2 := newMockSection("sec2")

	sec1.On("SetState", true).Return()
	sec1.On("SetState", false).Return()
	sec2.On("SetState", true).Return()
	sec2.On("SetState", false).Return()

	secRunner := NewSectionRunner()

	prog := NewProgram("test", []ProgItem{
		ProgItem{sec1, 25 * time.Millisecond},
	}, makeSchedule(), false)

	prog.Start(secRunner)

	time.Sleep(20 * time.Millisecond)
	ass.Equal(false, prog.running)
	time.Sleep(20 * time.Millisecond)
	ass.Equal(false, prog.running)

	newSeq := ProgSequenceJSON{
		ProgItemJSON{0, "25ms"},
		ProgItemJSON{1, "25ms"},
	}
	newSched := makeSchedule()
	err := prog.Update(NewProgramJSON("test2", newSeq, &newSched, true), []Section{sec1, sec2})
	require.NoError(t, err)

	ass.Equal("test2", prog.Name)
	require.Len(t, prog.Sequence, 2)
	ass.Equal(true, prog.Enabled)

	time.Sleep(20 * time.Millisecond)
	ass.Equal(true, prog.running)
	time.Sleep(60 * time.Millisecond)
	ass.Equal(false, prog.running)

	prog.Quit()

	sec1.AssertNumberOfCalls(t, "SetState", 2)
	sec2.AssertNumberOfCalls(t, "SetState", 2)

	newSeq[0].Section = 3
	err = prog.Update(ProgramJSON{nil, &newSeq, nil, nil, nil}, []Section{sec1, sec2})
	ass.Error(err)
}

func TestPrograms_JSON(t *testing.T) {
	ass := assert.New(t)
	sections := []Section {
		&RpioSection{}, &RpioSection{},
	}

	str := `[
	{
		"name": "p1", "sequence": []
	}, {
		"name": "p2", "sequence": [{"section": 0, "duration": "1m"}],
		"sched": {},
		"enabled": true
	}
	]`

	var psj ProgramsJSON
	err := json.Unmarshal([]byte(str), &psj)
	require.NoError(t, err)

	require.Len(t, psj, 2)

	ass.Equal("p1", *psj[0].Name)
	ass.Len(*psj[0].Sequence, 0)

	ass.Equal("p2", *psj[1].Name)
	require.Len(t, *psj[1].Sequence, 1)
	ass.Equal(0, (*psj[1].Sequence)[0].Section)
	ass.Equal("1m", (*psj[1].Sequence)[0].Duration)
	ass.Equal(true, *psj[1].Enabled)

	ps, err := psj.ToPrograms(sections)
	require.NoError(t, err)

	require.Len(t, ps, 2)
	ass.Equal("p1", ps[0].Name)
	ass.Len(ps[0].Sequence, 0)
	ass.Equal(false, ps[0].Enabled)

	ass.Equal("p2", ps[1].Name)
	require.Len(t, ps[1].Sequence, 1)
	ass.Equal(sections[0], ps[1].Sequence[0].Sec)
	ass.Equal("1m0s", ps[1].Sequence[0].Duration.String())
	ass.Equal(true, ps[1].Enabled)

	(*psj[1].Sequence)[0].Section = 3
	_, err = psj.ToPrograms(sections)
	ass.Error(err)

	psj, err = ProgramsToJSON(ps, sections)
	require.NoError(t, err)

	require.Len(t, psj, 2)

	ass.Equal("p1", *psj[0].Name)
	ass.Len(*psj[0].Sequence, 0)

	ass.Equal("p2", *psj[1].Name)
	require.Len(t, *psj[1].Sequence, 1)
	ass.Equal(0, (*psj[1].Sequence)[0].Section)
	ass.Equal("1m0s", (*psj[1].Sequence)[0].Duration)
	ass.Equal(true, *psj[1].Enabled)

	ps[1].Sequence[0].Sec = &RpioSection{}
	_, err = ProgramsToJSON(ps, sections)
	ass.Error(err)
}