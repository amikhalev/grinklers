package logic

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"git.amikhalev.com/amikhalev/grinklers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRpioSectionInit(t *testing.T) {
	util.Logger.Out = ioutil.Discard

	os.Setenv("RPI", "true")
	assert.Error(t, RpioSectionInit())
	RpioSectionCleanup()

	os.Setenv("RPI", "")
	assert.NoError(t, RpioSectionInit())
	assert.NoError(t, RpioSectionCleanup())
}

func TestRpioSection_JSON(t *testing.T) {
	os.Setenv("RPI", "")
	RpioSectionInit()

	ass := assert.New(t)
	var sec RpioSection

	err := json.Unmarshal([]byte(`{"name": "test1234", "pin": 4}`), &sec)
	require.NoError(t, err)
	ass.Equal("test1234", sec.Name())
	ass.Equal(4, int(sec.pin))

	bytes, err := json.Marshal(&sec)
	require.NoError(t, err)
	ass.Equal(`{"id":0,"name":"test1234","pin":4}`, string(bytes))

	sec.SetState(true)
	bytes, err = json.Marshal(&sec)
	require.NoError(t, err)
	ass.Equal(`{"id":0,"name":"test1234","pin":4}`, string(bytes))
}

func TestRpioSections_JSON(t *testing.T) {
	os.Setenv("RPI", "")
	RpioSectionInit()

	ass := assert.New(t)
	var secs RpioSections

	err := json.Unmarshal([]byte(`[{"name": "test1", "pin": 4},{"name": "test2", "pin": 5}]`), &secs)
	require.NoError(t, err)

	require.Len(t, secs, 2)
	ass.Equal("test1", secs[0].Name())
	ass.Equal("test2", secs[1].Name())

	err = json.Unmarshal([]byte(`{}`), &secs)
	ass.Error(err)
}

func TestRpioSection_Update(t *testing.T) {
	os.Setenv("RPI", "")
	RpioSectionInit()

	ass := assert.New(t)

	onUpdate := make(chan SecUpdate, 6)
	sec := NewRpioSection("test", 5)
	sec.SetOnUpdate(onUpdate)

	ass.Equal(false, sec.State())
	sec.SetState(true)

	select {
	case s := <-onUpdate:
		ass.Equal(&sec, s.Sec)
		ass.Equal(SecUpdateState, s.Type)
	default:
		ass.Fail("no update received")
	}

	ass.Equal(true, sec.State())
	sec.SetState(false)

	select {
	case s := <-onUpdate:
		ass.Equal(&sec, s.Sec)
		ass.Equal(SecUpdateState, s.Type)
	default:
		ass.Fail("no update received")
	}

	ass.Equal(false, sec.State())
}

func TestRpioSection_SetState(t *testing.T) {
	ass := assert.New(t)

	os.Setenv("RPI", "true")
	ass.Error(RpioSectionInit())

	sec := NewRpioSection("test", 5)

	ass.Panics(func() {
		sec.SetState(true)
	})
	ass.Panics(func() {
		sec.State()
	})
	ass.Panics(func() {
		sec.SetState(false)
	})
	ass.Panics(func() {
		sec.State()
	})
}

func TestRpioSection_Run(t *testing.T) {
	os.Setenv("RPI", "")
	RpioSectionInit()

	ass := assert.New(t)
	secRunner := NewSectionRunner()
	secRunner.Start(nil)
	defer secRunner.Quit()

	sec := NewRpioSection("test2", 6)

	ass.Equal(false, sec.State())
	go secRunner.RunSection(&sec, 50*time.Millisecond)
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
