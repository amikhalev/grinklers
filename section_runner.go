package main

import (
	"fmt"
	"github.com/inconshreveable/log15"
	"time"
)

type SectionRun struct {
	Sec      Section
	Duration time.Duration
	Done     chan<- int
}

func (sr *SectionRun) String() string {
	return fmt.Sprintf("{'%s' for %v}", sr.Sec.Name(), sr.Duration)
}

type SRQueue struct {
	items []*SectionRun
	head  int
	tail  int
}

func newSRQueue(size int) SRQueue {
	return SRQueue{
		make([]*SectionRun, size),
		0, 0,
	}
}

func (q *SRQueue) Push(item *SectionRun) {
	q.items[q.tail] = item
	itemsLen := len(q.items)
	q.tail = (q.tail + 1) % itemsLen
	if q.tail == q.head {
		// if queue is full, double storage size
		newItems := make([]*SectionRun, len(q.items)*2)
		copy(newItems, q.items[q.head:])
		copy(newItems[itemsLen-q.head:], q.items[:q.head])
		q.head = 0
		q.tail = itemsLen
		q.items = newItems
	}
}

func (q *SRQueue) Pop() *SectionRun {
	if q.head == q.tail {
		return nil
	}
	item := q.items[q.head]
	q.head = (q.head + 1) % len(q.items)
	if item == nil {
		return q.Pop()
	} else {
		return item
	}
}

func (q *SRQueue) Len() int {
	count := 0
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if q.items[i] != nil {
			count++
		}
	}
	return count
}

func (q *SRQueue) RemoveMatchingSection(sec Section) {
	checkAndRemove := func(i int) {
		if q.items[i] != nil && q.items[i].Sec == sec {
			q.items[i] = nil
		}
	}
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		checkAndRemove(i)
	}
	checkAndRemove(q.tail)
}

type SectionRunner struct {
	run    chan SectionRun
	cancel chan Section
	log15.Logger
}

func NewSectionRunner() SectionRunner {
	sr := SectionRunner{
		make(chan SectionRun, 2), make(chan Section, 2),
		logger.New(),
	}
	go sr.start()
	return sr
}

func (r *SectionRunner) start() {
	queue := newSRQueue(10)
	var (
		currentItem *SectionRun
		delay       <-chan time.Time
	)
	runItem := func() {
		if currentItem == nil {
			return
		}
		r.Debug("running section", "queueLen", queue.Len(), "currentItem", currentItem)
		currentItem.Sec.SetState(true)
		delay = time.After(currentItem.Duration)
	}
	finishRun := func() {
		currentItem.Sec.SetState(false)
		delay = nil
		if currentItem.Done != nil {
			currentItem.Done <- queue.Len()
		}
		currentItem = queue.Pop()
		r.Debug("finished running section", "queueLen", queue.Len(), "currentItem", currentItem)
	}
	for {
		select {
		case item := <-r.run:
			if currentItem == nil {
				currentItem = &item
				runItem()
			} else {
				queue.Push(&item)
				r.Debug("queued section run", "queueLen", queue.Len(), "currentItem", currentItem, "item", &item)
			}
		case cancelSec := <-r.cancel:
			queue.RemoveMatchingSection(cancelSec)
			if currentItem != nil && currentItem.Sec == cancelSec {
				finishRun()
				runItem()
			}
			r.Debug("cancelled section run", "queueLen", queue.Len(), "currentItem", currentItem, "sec", cancelSec.Name())
		case <-delay:
			finishRun()
			runItem()
		}
	}
}

func (r *SectionRunner) QueueSectionRun(sec Section, dur time.Duration) {
	r.run <- SectionRun{sec, dur, nil}
}

func (r *SectionRunner) RunSectionAsync(sec Section, dur time.Duration) <-chan int {
	done := make(chan int, 1)
	r.run <- SectionRun{sec, dur, done}
	return done
}

func (r *SectionRunner) RunSection(sec Section, dur time.Duration) {
	<-r.RunSectionAsync(sec, dur)
}

func (r *SectionRunner) CancelSection(sec Section) {
	r.cancel <- sec
}
