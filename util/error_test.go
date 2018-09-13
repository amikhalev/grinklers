package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
