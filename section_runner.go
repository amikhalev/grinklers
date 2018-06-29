package grinklers

import (
	"fmt"
	"time"

	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
)

const SR_ID_ALL = -1;

// SectionRunJSON is the JSON representation of a SectionRun
type SectionRunJSON struct {
	ID            int32      `json:"id"`
	Section       int        `json:"section"`
	TotalDuration float64    `json:"totalDuration"`
	Duration      float64    `json:"duration"`
	StartTime     *time.Time `json:"startTime"`
	PauseTime     *time.Time `json:"pauseTime"`
	UnpauseTime   *time.Time `json:"unpauseTime"`
}

// SRStateJSON is the JSON representation of a SRState
type SRStateJSON struct {
	Queue   []SectionRunJSON `json:"queue"`
	Current *SectionRunJSON  `json:"current"`
	Paused  bool             `json:"paused"`
}

// SectionRun is a single run of a section for a duration that is either queued, or currently running
type SectionRun struct {
	// RunID is a sequential unique identifier of SectionRuns
	RunID int32
	// Sec is the section that should be run
	Sec Section
	// Duration is the total duration the section is to run for
	TotalDuration time.Duration
	// Duration is the remaining duration the section is to run for. This should equal TotalDuration unless the section
	// is currently paused, or has been paused
	Duration time.Duration
	// Done is a chan that a value is sent on when the section is done running. This value is true
	// if the section run was cancelled, and false if it finished normally
	Done chan<- bool
	// StartTime is the time the section started running, or nil if the section is still queued.
	StartTime *time.Time
	// PauseTime is the time the section was paused, if the SectionRunner is currently paused. Otherwise
	// it is nil
	PauseTime *time.Time
	// UnpauseTime is the time the section was unpaused, if it was paused and then unpaused. Otherwise, nil
	UnpauseTime *time.Time
}

// NewSectionRun creates a new SectionRun
func NewSectionRun(runID int32, sec Section, duration time.Duration, doneChan chan<- bool) SectionRun {
	return SectionRun{
		runID, sec, duration, duration, doneChan,
		nil, nil, nil,
	}
}

func (sr *SectionRun) String() string {
	if sr == nil {
		return "nil"
	}
	return fmt.Sprintf("{'%s' for %v}", sr.Sec.Name(), sr.Duration)
}

// ToJSON returns an the JSON representation of this SectionRun, or err if there was some error.
// sections is a slice of the sections (in order to compute the section index)
func (sr *SectionRun) ToJSON(sections []Section) (j SectionRunJSON, err error) {
	secID := -1
	for i, sec := range sections {
		if sr.Sec == sec {
			secID = i
		}
	}
	if secID == -1 {
		err = fmt.Errorf("the section of this program does not exist in the sections array")
		return
	}
    j = SectionRunJSON{
    	sr.RunID, secID, sr.TotalDuration.Seconds(), sr.Duration.Seconds(),
    	sr.StartTime, sr.PauseTime, sr.UnpauseTime,
	}
	return
}

// SRQueue is a queue for SectionRuns. It is implemented as a circular buffer that doubles in length when it fills up
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

// Format implements Formatter for SrQueue
func (q SRQueue) Format(f fmt.State, c rune) {
	fmt.Fprint(f, "[")
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if i != q.head {
			fmt.Fprint(f, ", ")
		}
		if q.items[i] != nil {
			fmt.Fprint(f, q.items[i])
		}
	}
	fmt.Fprint(f, "]")
}

var _ fmt.Formatter = (*SRQueue)(nil)

// ToJSON returns the JSON representation of this SRQueue, or err if there was an error.
// See SectionRun#toJSON
func (q *SRQueue) ToJSON(sections []Section) (slice []SectionRunJSON, err error) {
	slice = nil
	var json SectionRunJSON
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if q.items[i] != nil {
			json, err = q.items[i].ToJSON(sections)
			if err != nil {
				return
			}
			slice = append(slice, json)
		}
	}
	return
}

// Push adds an item to the end of the SrQueue, expanding it if necessary
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

// Pop pops the first item off the SrQueue
func (q *SRQueue) Pop() *SectionRun {
	if q.head == q.tail {
		return nil
	}
	item := q.items[q.head]
	q.head = (q.head + 1) % len(q.items)
	if item == nil {
		return q.Pop()
	}
	return item
}

// Len gets the current number of items in the SrQueue
func (q *SRQueue) Len() int {
	count := 0
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if q.items[i] != nil {
			count++
		}
	}
	return count
}

// RemoveWithSection removes all items from the queue that are runs with the specified section
func (q *SRQueue) RemoveWithSection(sec Section) (removed []*SectionRun) {
	removed = make([]*SectionRun, 0)
	checkAndRemove := func(i int) {
		if q.items[i] != nil && q.items[i].Sec == sec {
			removed = append(removed, q.items[i])
			q.items[i] = nil
		}
	}
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		checkAndRemove(i)
	}
	checkAndRemove(q.tail)
	return
}

// RemoveByID removes the SectionRun with the specified id and returns it, or returns nil if none existed
func (q *SRQueue) RemoveByID(id int32) *SectionRun {
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		if q.items[i] != nil && q.items[i].RunID == id {
			item := q.items[i]
			q.items[i] = nil
			return item
		}
	}
	return nil
}

// RemoveAll removes all section from the queue and returns them
func (q SRQueue) RemoveAll() (removed []*SectionRun) {
	removed = make([]*SectionRun, 0)
	checkAndRemove := func(i int) {
		if q.items[i] != nil {
			removed = append(removed, q.items[i])
			q.items[i] = nil
		}
	}
	for i := q.head; i != q.tail; i = (i + 1) % len(q.items) {
		checkAndRemove(i)
	}
	checkAndRemove(q.tail)
	return
}

// SRState is the state of the SectionRunner. All accesses synchronized over Mu
type SRState struct {
	Queue   SRQueue
	Current *SectionRun
	Paused  bool
	sync.Mutex // gives it Lock() and Unlock methods
}

func newSRState() SRState {
	return SRState{
		newSRQueue(10), nil, false, sync.Mutex{},
	}
}

// ToJSON returns the JSON representation of a SRState, or an error
func (s *SRState) ToJSON(sections []Section) (json SRStateJSON, err error) {
	json = SRStateJSON{}
	json.Queue, err = s.Queue.ToJSON(sections)
	if err != nil {
		return
	}
	if s.Current != nil {
		var current SectionRunJSON
		current, err = s.Current.ToJSON(sections)
		json.Current = &current
	} else {
		json.Current = nil
	}
	json.Paused = s.Paused
	return
}

func (s *SRState) String() string {
	return fmt.Sprintf("{Current: %v, Queue: %v, Paused: %t}", s.Current, s.Queue, s.Paused)
}

// SectionRunner runs a queue of sections
type SectionRunner struct {
	run           chan SectionRun
	cancelSec     chan Section
	cancelID      chan int32
	paused        chan bool
	quit          chan struct{}
	nextID        int32
	State         SRState
	OnUpdateState chan<- *SRState
	log           *logrus.Entry
}

// NewSectionRunner creates a new SectionRunner without starting it
func NewSectionRunner() *SectionRunner {
	return &SectionRunner{
		make(chan SectionRun, 2), make(chan Section, 2), make(chan int32, 2),
		make(chan bool, 2), make(chan struct{}),
		0, newSRState(), nil,
		Logger.WithField("module", "SectionRunner"),
	}
}

func (r *SectionRunner) start(wait *sync.WaitGroup) {
	r.stateUpdate()
	state := &r.State
	endUpdate := func() {
		r.State.Unlock()
		r.stateUpdate()
	}
	var (
		delay <-chan time.Time
	)
	runItem := func() {
		if state.Current == nil {
			return
		}
		r.log.WithField("state", state).Info("running section")
		startTime := time.Now()
		state.Current.StartTime = &startTime
		if state.Paused {
			delay = nil
			state.Current.PauseTime = &startTime
		} else {
			state.Current.Sec.SetState(true)
			delay = time.After(state.Current.Duration)
		}
	}
	finishRun := func(cancelled bool) {
		state.Current.Sec.SetState(false)
		delay = nil
		if state.Current.Done != nil {
			state.Current.Done <- cancelled
		}
		var verb string
		if cancelled {
			verb = "cancelled"
		} else {
			verb = "finished"
		}
		r.log.WithField("state", state).Infof("%s running section", verb)
		state.Current = state.Queue.Pop()
	}
	if wait != nil {
		defer wait.Done()
	}
	for {
		select {
		case <-r.quit:
			r.log.Debug("quiting section runner")
			return
		case item := <-r.run:
			state.Lock()
			if state.Current == nil && !state.Paused {
				state.Current = &item
				runItem()
			} else {
				state.Queue.Push(&item)
				r.log.WithField("state", state).Debug("queued section run")
			}
			endUpdate()
		case sec := <-r.cancelSec:
			state.Lock()
			fromQueue := state.Queue.RemoveWithSection(sec)
			for _, secRun := range fromQueue {
				if secRun.Done != nil {
					secRun.Done <- true
				}
			}
			if state.Current != nil && state.Current.Sec == sec {
				finishRun(true)
				runItem()
			}
			r.log.WithFields(logrus.Fields{
				"state": state, "sec": sec.Name(),
			}).Debug("cancelled section runs with section")
			endUpdate()
		case id := <-r.cancelID:
			state.Lock()
			if id == SR_ID_ALL {
				fromQueue := state.Queue.RemoveAll()
				for _, secRun := range fromQueue {
					if secRun.Done != nil {
						secRun.Done <- true
					}
				}
				if state.Current != nil {
					finishRun(true)
					runItem()
				}
				r.log.WithFields(logrus.Fields{
					"state": state,
				}).Debug("cancelled all section runs")
			} else {
				fromQueue := state.Queue.RemoveByID(id)
				if fromQueue != nil && fromQueue.Done != nil {
					fromQueue.Done <- true
				}
				if state.Current != nil && state.Current.RunID == id {
					finishRun(true)
					runItem()
				}
				r.log.WithFields(logrus.Fields{
					"state": state, "id": id,
				}).Debug("cancelled section run by id")
			}
			endUpdate()
		case paused := <-r.paused:
			state.Lock()
			if state.Paused == paused { // no change necessary
				state.Unlock()
				break
			}
			now := time.Now()
			if paused {
				var alreadyRunFor time.Duration
				if state.Current != nil {
					if state.Current.UnpauseTime != nil {
						alreadyRunFor = now.Sub(*state.Current.UnpauseTime)
					} else {
						alreadyRunFor = now.Sub(*state.Current.StartTime)
					}
					state.Current.Sec.SetState(false)
					state.Current.PauseTime = &now
					state.Current.Duration = state.Current.Duration - alreadyRunFor
					delay = nil // so it never receives
				}
				state.Paused = true
				r.log.WithFields(logrus.Fields{
					"alreadyRunFor": alreadyRunFor,
					"state": state,
				}).Debug("paused section runner")
			} else {
				state.Paused = false
				if state.Current != nil {
					r.log.WithFields(logrus.Fields{
						"remaining":     state.Current.Duration,
						"run":           state.Current,
					}).Debug("resuming paused section")
					delay = time.After(state.Current.Duration)
					state.Current.PauseTime = nil
					state.Current.UnpauseTime = &now
					state.Current.Sec.SetState(true)
				} else {
					state.Current = state.Queue.Pop()
					runItem()
					r.log.WithField("state", state).Debug("unpaused section runner")
				}
			}
			endUpdate()
		case <-delay:
			state.Lock()
			finishRun(false)
			runItem()
			endUpdate()
		}
	}
}

func (r *SectionRunner) stateUpdate() {
	if r.OnUpdateState != nil {
		r.OnUpdateState <- &r.State
	}
}

// Start starts the background goroutine of a SectionRunner
func (r *SectionRunner) Start(wait *sync.WaitGroup) {
	if wait != nil {
		wait.Add(1)
	}
	go r.start(wait)
}

// Quit tells the background goroutine to stop
func (r *SectionRunner) Quit() {
	r.quit <- struct{}{}
}

func (r *SectionRunner) getNextID() int32 {
	return atomic.AddInt32(&r.nextID, 1) - 1
}

// QueueSectionRun queues the specified Section to run for dur
func (r *SectionRunner) QueueSectionRun(sec Section, dur time.Duration) (id int32) {
	id = r.getNextID()
	r.run <- NewSectionRun(id, sec, dur, nil)
	return
}

// RunSectionAsync runs the section and returns a chan which recieves when the section is finished running
func (r *SectionRunner) RunSectionAsync(sec Section, dur time.Duration) (id int32, done <-chan bool) {
	id = r.getNextID()
	doneChan := make(chan bool, 1)
	r.run <- NewSectionRun(id, sec, dur, doneChan)
	done = doneChan
	return
}

// RunSection runs the section and returns when the section is finished running
func (r *SectionRunner) RunSection(sec Section, dur time.Duration) {
	_, done := r.RunSectionAsync(sec, dur)
	<-done
}

// CancelSection cancels all runs for the specified Section
func (r *SectionRunner) CancelSection(sec Section) {
	r.cancelSec <- sec
}

// CancelID cancels the section run with the specified id
func (r *SectionRunner) CancelID(id int32) {
	r.cancelID <- id
}

// CancelAll cancels all section runs
func (r *SectionRunner) CancelAll() {
	r.cancelID <- SR_ID_ALL
}

// Pause pauses the currently running section run (if any) and stops processing the section run queue
func (r *SectionRunner) Pause() {
	r.paused <- true
}

// Unpause resumes both the paused section (if any) and processing of the section run queue
func (r *SectionRunner) Unpause() {
	r.paused <- false
}
