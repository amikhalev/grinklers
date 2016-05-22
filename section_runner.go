package main

import (
	"time"
	log "github.com/inconshreveable/log15"
)

type SectionRun struct {
	Sec      *Section
	Duration time.Duration
	Done     chan int
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
		newItems := make([]*SectionRun, len(q.items) * 2)
		copy(newItems, q.items[q.head:])
		copy(newItems[itemsLen - q.head:], q.items[:q.head])
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
	return item
}

func (q *SRQueue) Len() int {
	length := q.tail - q.head
	if length < 0 {
		length += len(q.items)
	}
	return length
}

type SectionRunner struct {
	run    chan SectionRun
	cancel chan *Section
	log.Logger
}

func NewSectionRunner() SectionRunner {
	sr := SectionRunner{
		make(chan SectionRun, 2), make(chan *Section, 2),
		log.New(),
	}
	go sr.start()
	return sr
}

func (r *SectionRunner) start() {
	queue := newSRQueue(7)
	var (
		currentItem *SectionRun; delay <-chan time.Time
	)
	runItem := func() {
		if currentItem == nil {
			return
		}
		r.Debug("running section")
		currentItem.Sec.On()
		delay = time.After(currentItem.Duration)
	}
	finishRun := func () {
		currentItem.Sec.Off()
		log.Info("finished running section")
		if currentItem.Done != nil {
			currentItem.Done <- queue.Len()
		}
		currentItem = queue.Pop()
	}
	for {
		select {
		case item := <-r.run:
			if currentItem == nil {
				currentItem = &item
				runItem()
			} else {
				queue.Push(&item)
			}
		case cancelSec := <-r.cancel:
			if currentItem.Sec == cancelSec {
				finishRun()
			}
		case <-delay:
			finishRun()
			runItem()
		}
	}
}

func (r *SectionRunner) QueueSectionRun(sec *Section, dur time.Duration) {
	r.run <- SectionRun{ sec, dur, nil }
}

func (r *SectionRunner) RunSectionAsync(sec *Section, dur time.Duration) <-chan int {
	done := make(chan int, 1)
	r.run <- SectionRun{ sec, dur, done }
	return done
}

func (r *SectionRunner) RunSection(sec *Section, dur time.Duration) {
	<-r.RunSectionAsync(sec, dur)
}

func (r *SectionRunner) CancelSection(sec *Section) {
	r.cancel <- sec
}