package main

import (
	"encoding/json"
	"github.com/inconshreveable/log15"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestInitSection(t *testing.T) {
	logger.SetHandler(log15.DiscardHandler())

	os.Setenv("RPI", "true")
	assert.Panics(t, InitSection)
	CleanupSection()

	os.Setenv("RPI", "")
	assert.NotPanics(t, InitSection)
	assert.NotPanics(t, CleanupSection)
}

func TestRpioSection_JSON(t *testing.T) {
	os.Setenv("RPI", "")
	InitSection()

	ass := assert.New(t)
	var sec RpioSection

	err := json.Unmarshal([]byte(`{"name": "test1234", "pin": 4}`), &sec)
	require.NoError(t, err)
	ass.Equal("test1234", sec.Name())
	ass.Equal(4, int(sec.pin))

	bytes, err := json.Marshal(&sec)
	require.NoError(t, err)
	ass.Equal(`{"name":"test1234","pin":4,"state":false}`, string(bytes))

	sec.SetState(true)
	bytes, err = json.Marshal(&sec)
	require.NoError(t, err)
	ass.Equal(`{"name":"test1234","pin":4,"state":true}`, string(bytes))
}

func TestRpioSection_Update(t *testing.T) {
	os.Setenv("RPI", "")
	InitSection()

	ass := assert.New(t)

	onUpdate := make(chan *RpioSection, 6)
	sec := NewRpioSection("test", 5)
	sec.OnUpdate = onUpdate

	ass.Equal(false, sec.State())
	sec.SetState(true)

	select {
	case s := <-onUpdate:
		ass.Equal(&sec, s)
	default:
		ass.Fail("no update received")
	}

	ass.Equal(true, sec.State())
	sec.SetState(false)

	select {
	case s := <-onUpdate:
		ass.Equal(&sec, s)
	default:
		ass.Fail("no update received")
	}

	ass.Equal(false, sec.State())
}

func TestRpioSection_Run(t *testing.T) {
	os.Setenv("RPI", "")
	InitSection()

	ass := assert.New(t)
	sectionRunner = NewSectionRunner()
	sec := NewRpioSection("test2", 6)

	ass.Equal(false, sec.State())
	go sec.RunFor(50 * time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	ass.Equal(true, sec.State())
	time.Sleep(50 * time.Millisecond)
	ass.Equal(false, sec.State())

	ass.Equal(false, sec.State())
	go sec.RunFor(time.Minute)
	time.Sleep(25 * time.Millisecond)
	ass.Equal(true, sec.State())
	sec.Cancel()
	time.Sleep(25 * time.Millisecond)
	ass.Equal(false, sec.State())

}
