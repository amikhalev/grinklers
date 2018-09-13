package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAtomicBool(t *testing.T) {
	ass := assert.New(t)

	b := NewAtomicBool(true)
	ass.Equal(true, b.Load())
	ass.NotPanics(func() {
		b.Store(false)
	})
	ass.Equal(false, b.Load())

	ass.Equal(true, b.StoreIf(false, true))
	ass.Equal(true, b.Load())
	ass.Equal(true, b.StoreIf(true, false))
	ass.Equal(false, b.Load())
	ass.Equal(false, b.StoreIf(true, false))
	ass.Equal(false, b.Load())
	b.Store(true)
	ass.Equal(false, b.StoreIf(false, true))
	ass.Equal(true, b.Load())
}

func TestAtomicBool_Concurrent(t *testing.T) {
	var (
		counter int
		ab      = NewAtomicBool(false)
	)
	routine := func() {
		if ab.StoreIf(false, true) {
			counter = counter + 1
		}
	}
	for i := 0; i < 100; i++ {
		routine()
	}
}

func BenchmarkAtomicBool(b *testing.B) {
	ab := NewAtomicBool(false)

	for i := 0; i < b.N; i++ {
		ab.Store(!ab.Load())
	}
}
