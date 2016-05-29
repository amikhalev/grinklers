package grinklers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExhaustChan(t *testing.T) {
	ass := assert.New(t)

	ch := make(chan interface{}, 10)

	for i := 0; i < 10; i++ {
		ch <- nil
	}

	ass.NotPanics(func() {
		ExhaustChan(ch)
	})
	select {
	case <-ch:
		ass.Fail("channel not exhausted")
	default:

	}

	ass.Panics(func() {
		ExhaustChan("not a chan lel")
	})
}

func TestCheckNotNil(t *testing.T) {
	ass := assert.New(t)
	ass.Error(CheckNotNil(nil, "test"))
	ass.NoError(CheckNotNil("not nil", "test"))
}

func TestCheckRange(t *testing.T) {
	ass := assert.New(t)

	num := 10
	num2 := -3

	ass.NoError(CheckRange(&num, "test", 11))
	ass.Error(CheckRange(nil, "test", 11))
	ass.Error(CheckRange(&num, "test", 6))
	ass.Error(CheckRange(&num2, "test", 6))
	ass.Error(CheckRange(&num, "test", 10))
}
