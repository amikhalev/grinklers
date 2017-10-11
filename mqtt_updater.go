package grinklers

import (
	"github.com/Sirupsen/logrus"
)

// MQTTUpdater updates MQTT topics with the current state of the application
type MQTTUpdater struct {
	config                *ConfigData
	onSectionUpdate       chan SecUpdate
	onProgramUpdate       chan ProgUpdate
	onSectionRunnerUpdate chan *SRState
	stop                  chan int
	api                   *MQTTApi
	logger                *logrus.Entry
}

// NewMQTTUpdater creates a new MQTTUpdater which uses the specified state
func NewMQTTUpdater(config *ConfigData, sectionRunner *SectionRunner) *MQTTUpdater {
	onSectionUpdate := make(chan SecUpdate, 10)
	onProgramUpdate := make(chan ProgUpdate, 10)
	onSectionRunnerUpdate := make(chan *SRState, 10)
	stop := make(chan int)
	for i := range config.Sections {
		config.Sections[i].SetOnUpdate(onSectionUpdate)
	}
	for i := range config.Programs {
		config.Programs[i].OnUpdate = onProgramUpdate
	}
	sectionRunner.OnUpdateState = onSectionRunnerUpdate
	return &MQTTUpdater{
		config,
		onSectionUpdate, onProgramUpdate, onSectionRunnerUpdate, stop, nil,
		Logger.WithField("module", "MQTTUpdater"),
	}
}

// UpdateSections updates the topics for all sections
func (u *MQTTUpdater) UpdateSections() {
	u.api.UpdateSections(u.config.Sections)
}

// UpdatePrograms updates topics for all programs
func (u *MQTTUpdater) UpdatePrograms() {
	u.api.UpdatePrograms(u.config.Programs)
}

func (u *MQTTUpdater) run() {
	u.logger.Debug("starting updater")
	u.UpdateSections()
	u.UpdatePrograms()
	for {
		//logger.Debug("waiting for update")
		select {
		case <-u.stop:
			return
		case secUpdate := <-u.onSectionUpdate:
			//logger.Debug("sec update")
			ExhaustChan(u.onSectionUpdate)

			index := -1
			for i, s := range u.config.Sections {
				if s == secUpdate.Sec {
					index = i
				}
			}
			if index == -1 {
				u.logger.Panicf("invalid section update recieved: %v", secUpdate.Sec)
			}

			var err error
			switch secUpdate.Type {
			case SecUpdateData:
				err = u.api.UpdateSectionData(index, secUpdate.Sec)
				if err == nil {
					err = WriteConfig(u.config)
				}
			case SecUpdateState:
				err = u.api.UpdateSectionState(index, secUpdate.Sec)
			default:
			}
			if err != nil {
				u.logger.WithError(err).Error("error updating sections")
			}
		case progUpdate := <-u.onProgramUpdate:
			//logger.Debug("prog update")
			ExhaustChan(u.onProgramUpdate)

			index := -1
			for i := range u.config.Programs {
				if &u.config.Programs[i] == progUpdate.Prog {
					index = i
				}
			}
			if index == -1 {
				u.logger.Panicf("invalid program update recieved: %v", progUpdate.Prog)
			}

			var err error
			switch progUpdate.Type {
			case ProgUpdateData:
				err = u.api.UpdateProgramData(index, progUpdate.Prog)
				if err == nil {
					err = WriteConfig(u.config)
				}
			case pupdateRunning:
				err = u.api.UpdateProgramRunning(index, progUpdate.Prog)
			default:
			}
			if err != nil {
				u.logger.WithError(err).Error("error updating sections")
			}
		case srState := <-u.onSectionRunnerUpdate:
			ExhaustChan(u.onSectionRunnerUpdate)
			srState.Lock()
			u.logger.WithField("srState", srState).Debugf("section runner update")
			srState.Unlock()

			err := u.api.UpdateSectionRunner(srState)
			if err != nil {
				u.logger.WithError(err).Error("error updating section runner state")
			}
		}
	}
}

// Start starts the MQTTUpdater to listen and update topics
func (u *MQTTUpdater) Start(api *MQTTApi) {
	u.api = api
	go u.run()
}

// Stop stops the updater from updating topics
func (u *MQTTUpdater) Stop() {
	u.stop <- 0
}
