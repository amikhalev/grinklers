package grinklers

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
	Logger.SetHandler(log15.DiscardHandler())

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

func TestRpioSections_JSON(t *testing.T) {
	os.Setenv("RPI", "")
	InitSection()

	ass := assert.New(t)
	var secs RpioSections

	err := json.Unmarshal([]byte(`[{"name": "test1", "pin": 4},{"name": "test2", "pin": 5}]`), &secs)
	require.NoError(t, err)

	require.Len(t, secs, 2)
	ass.Equal("test1", secs[0].Name())
	ass.Equal("test2", secs[1].Name())
}

func TestRpioSection_Update(t *testing.T) {
	os.Setenv("RPI", "")
	InitSection()

	ass := assert.New(t)

	onUpdate := make(chan Section, 6)
	sec := NewRpioSection("test", 5)
	sec.SetOnUpdate(onUpdate)

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
	secRunner := NewSectionRunner()
	sec := NewRpioSection("test2", 6)

	ass.Equal(false, sec.State())
	go secRunner.RunSection(&sec, 50 * time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	ass.Equal(true, sec.State())
	time.Sleep(50 * time.Millisecond)
	ass.Equal(false, sec.State())

	ass.Equal(false, sec.State())
	go secRunner.RunSection(&sec, time.Minute)
	time.Sleep(25 * time.Millisecond)
	ass.Equal(true, sec.State())
	secRunner.CancelSection(&sec)
	time.Sleep(25 * time.Millisecond)
	ass.Equal(false, sec.State())

}
