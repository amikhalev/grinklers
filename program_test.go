package main

import (
	"encoding/json"
	"github.com/amikhalev/grinklers/sched"
	"github.com/inconshreveable/log15"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestProgram_JSON(t *testing.T) {
	logger.SetHandler(log15.DiscardHandler())
	ass := assert.New(t)
	var prog Program

	configData.Sections = []RpioSection{
		RpioSection{}, RpioSection{},
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
	err := json.Unmarshal([]byte(str), &prog)
	require.NoError(t, err)
	ass.Equal("test 1234", prog.Name)
	ass.Equal(true, prog.Enabled)
	ass.Equal(ProgItem{&configData.Sections[0], 1*time.Hour + 2*time.Minute + 3*time.Second}, prog.Sequence[0])
	ass.Equal(ProgItem{&configData.Sections[1], 24 * time.Millisecond}, prog.Sequence[1])
	ass.Equal(sched.TimeOfDay{1, 2, 0, 0}, prog.Sched.Times[0])
	ass.Contains(prog.Sched.Weekdays, time.Monday)
	ass.Contains(prog.Sched.Weekdays, time.Wednesday)
	ass.Contains(prog.Sched.Weekdays, time.Friday)

	_, err = json.Marshal(&prog)
	require.NoError(t, err)
}

func TestProgram_Run(t *testing.T) {
	sec1 := newMockSection("sec1")
	sec2 := newMockSection("sec2")

	sectionRunner = NewSectionRunner()

	onUpdate := make(chan *Program, 10)

	prog := NewProgram("test", []ProgItem{
		ProgItem{sec1, 10 * time.Millisecond},
		ProgItem{sec2, 10 * time.Millisecond},
	}, sched.Schedule{}, false)
	prog.OnUpdate = onUpdate
	prog.Start()

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
	assert.Equal(t, &prog, p)
	assert.Equal(t, true, prog.running)

	p = <-onUpdate
	assert.Equal(t, &prog, p)
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

	sectionRunner = NewSectionRunner()

	runTime := time.Now().Add(25 * time.Millisecond)
	prog := NewProgram("test", []ProgItem{
		ProgItem{sec1, 25 * time.Millisecond},
		ProgItem{sec2, 25 * time.Millisecond},
	}, sched.Schedule{
		Times: []sched.TimeOfDay{
			sched.TimeOfDay{runTime.Hour(), runTime.Minute(), runTime.Second(), runTime.Nanosecond() / 1000000},
		},
		Weekdays: []time.Weekday{0, 1, 2, 3, 4, 5, 6},
	}, true)
	prog.Start()

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, true, prog.running)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, false, prog.running)

	prog.Quit()
}
