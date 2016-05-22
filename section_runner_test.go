package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"time"
)

func TestSRQueue(t *testing.T) {
	ass := assert.New(t)
	queue := newSRQueue(2)
	ass.NotNil(queue)

	item1 := &SectionRun{nil, 5 * time.Second}
	item2 := &SectionRun{nil, 10 * time.Second}
	item3 := &SectionRun{nil, 15 * time.Second}

	ass.Nil(queue.Pop(), "Pop() should be nil when empty")
	ass.Equal(0, queue.Len(), "Len() should be 0 when empty")

	queue.Push(item1)
	ass.Equal(1, queue.Len(), "Len() should be 1")
	queue.Push(item2)
	ass.Equal(2, queue.Len(), "Len() should be 2")
	queue.Push(item3)
	ass.Equal(3, queue.Len(), "Len() should be 3")

	ass.Equal(item1, queue.Pop(), "item1 is not 1 out of queue")
	ass.Equal(item2, queue.Pop(), "item2 is not 2 out of queue")
	ass.Equal(item3, queue.Pop(), "item3 is not 3 out of queue")
	ass.Equal(0, queue.Len(), "Len() should be 0")

	queue.Push(item1)
	ass.Equal(1, queue.Len(), "Len() should be 1")
	ass.Equal(item1, queue.Pop(), "item1 is not out of queue")
	ass.Equal(0, queue.Len(), "Len() should be 0")
	queue.Push(item2)
	ass.Equal(1, queue.Len(), "Len() should be 1")
	ass.Equal(item2, queue.Pop(), "item2 is not out of queue")
	ass.Equal(0, queue.Len(), "Len() should be 0")
	queue.Push(item3)
	ass.Equal(1, queue.Len(), "Len() should be 1")
	ass.Equal(item3, queue.Pop(), "item3 is not out of queue")
	ass.Equal(0, queue.Len(), "Len() should be 0")
}
