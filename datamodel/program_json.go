package datamodel

import (
	"fmt"
	"time"

	"git.amikhalev.com/amikhalev/grinklers/logic"
	"git.amikhalev.com/amikhalev/grinklers/sched"
	"git.amikhalev.com/amikhalev/grinklers/util"
)

// ProgItemJSON is the JSON representation of a ProgItem
type ProgItemJSON struct {
	Section int `json:"section"`
	// Duration of the program item in seconds
	Duration float64 `json:"duration"`
}

// ToProgItem converts a ProgItemJSON to a ProgItem
func (data *ProgItemJSON) ToProgItem(sections []logic.Section) (pi *logic.ProgItem, err error) {
	dur := time.Duration(data.Duration * float64(time.Second))
	if err = util.CheckRange(&data.Section, "section id", len(sections)); err != nil {
		err = fmt.Errorf("invalid program item section id: %v", err)
		return
	}
	pi = &logic.ProgItem{Sec: &sections[data.Section], Duration: dur}
	return
}

// ProgItemToJSON converts a ProgItem to a ProgItemJSON
func ProgItemToJSON(pi *logic.ProgItem) ProgItemJSON {
	return ProgItemJSON{pi.Sec.ID, pi.Duration.Seconds()}
}

// ProgSequenceJSON is a sequence of ProgItemJSONs
type ProgSequenceJSON []ProgItemJSON

// ProgSequenceToJSON converts a ProgSequence to a ProgSequenceJSON
func ProgSequenceToJSON(seq logic.ProgSequence) (seqj ProgSequenceJSON) {
	seqj = make(ProgSequenceJSON, len(seq))
	for i := range seq {
		seqj[i] = ProgItemToJSON(&seq[i])
	}
	return
}

// ToSequence converts a ProgSequenceJSON to a ProgSequence
func (seqj ProgSequenceJSON) ToSequence(sections []logic.Section) (seq logic.ProgSequence, err error) {
	seq = make(logic.ProgSequence, len(seqj))
	var pi *logic.ProgItem
	for i := range seqj {
		pi, err = seqj[i].ToProgItem(sections)
		if err != nil {
			return
		}
		seq[i] = *pi
	}
	return
}

// ProgramJSON is the JSON representation of a Program
type ProgramJSON struct {
	ID       int              `json:"id"`
	Name     *string          `json:"name"`
	Sequence ProgSequenceJSON `json:"sequence"`
	Sched    *sched.Schedule  `json:"schedule"`
	Enabled  *bool            `json:"enabled"`
}

// NewProgramJSON creates a new ProgramJSON with the specified data
func NewProgramJSON(name *string, sequence ProgSequenceJSON, sched *sched.Schedule, enabled *bool) ProgramJSON {
	return ProgramJSON{
		0, name, sequence, sched, enabled,
	}
}

// ToProgram converts a ProgramJSON to a Program
func (data *ProgramJSON) ToProgram(sections []logic.Section) (prog *logic.Program, err error) {
	var (
		sequence []logic.ProgItem
		schedule = sched.Schedule{}
		enabled  = false
	)
	if err = util.CheckNotNil(data.Name, "name"); err != nil {
		return
	}
	sequence, err = data.Sequence.ToSequence(sections)
	if err != nil {
		return
	}
	if data.Sched != nil {
		schedule = *data.Sched
	}
	if data.Enabled != nil {
		enabled = *data.Enabled
	}
	// id will be assigned later
	prog = logic.NewProgram(*data.Name, sequence, schedule, enabled)
	return
}

// Update updates the data for this program based on the specified ProgramJSON, notifying
// the runner of any changes.
func (data *ProgramJSON) Update(prog *logic.Program, sections []logic.Section) (err error) {
	prog.Lock()
	defer prog.Unlock()
	if data.Name != nil {
		prog.Name = *data.Name
	}
	if data.Sequence != nil {
		sequence, err := data.Sequence.ToSequence(sections)
		if err != nil {
			return err
		}
		prog.Sequence = sequence
	}
	if data.Sched != nil {
		prog.Sched = *data.Sched
	}
	if data.Enabled != nil {
		prog.Enabled = *data.Enabled
	}
	prog.Refresh()
	prog.OnUpdate(logic.ProgUpdateData)
	return
}

// ProgramToJSON locks and converts a Program to a ProgramJSON
func ProgramToJSON(prog *logic.Program) (data ProgramJSON) {
	prog.Lock()
	defer prog.Unlock()
	sequence := ProgSequenceToJSON(prog.Sequence)
	return ProgramJSON{prog.ID, &prog.Name, sequence, &prog.Sched, &prog.Enabled}
}

// ProgramsJSON represents multiple ProgramJSONs in a JSON array
type ProgramsJSON []ProgramJSON

// ToPrograms converts this ProgramsJSON to Programs
func (progs ProgramsJSON) ToPrograms(sections []logic.Section) (programs []*logic.Program, err error) {
	var p *logic.Program
	for i := range progs {
		p, err = progs[i].ToProgram(sections)
		if err != nil {
			return
		}
		p.ID = i
		programs = append(programs, p)
	}
	return
}

// ProgramsToJSON converts programs to ProgramsJSON
func ProgramsToJSON(programs []*logic.Program) (data ProgramsJSON) {
	data = make(ProgramsJSON, len(programs))
	for i := range programs {
		data[i] = ProgramToJSON(programs[i])
	}
	return
}
