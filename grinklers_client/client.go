package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/eclipse/paho.mqtt.golang"
)

var (
	clientID = flag.String("cid", "grinklers_client", "The MQTT client ID to connect with")
)

var (
	timeoutPeriod = 100 * time.Millisecond
	timeoutError  = errors.New("the operation timed out")
)

type Section struct {
	Id   int
	Name string
	Pin  int
}

type GrinklersClient struct {
	mqttClient mqtt.Client
	prefix     string

	chanConnected   chan bool
	chanNumSections chan int
	chanSections    chan Section

	connected   bool
	numSections int
	sections    []Section
}

func NewGrinklersClient(mqttClient mqtt.Client, prefix string) *GrinklersClient {
	chanConnected := make(chan bool, 1)
	chanNumSections := make(chan int, 1)
	chanSections := make(chan Section, 1)
	return &GrinklersClient{
		mqttClient, prefix,
		chanConnected, chanNumSections, chanSections,
		false, -1, nil,
	}
}

func (c *GrinklersClient) Connect() {
	if token := c.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.WithError(token.Error()).Fatal("error connecting to mqtt broker")
		return
	}
	c.subscribe()
}

func (c *GrinklersClient) Disconnect() {
	c.mqttClient.Disconnect(250)
}

func (c *GrinklersClient) subscribe() {
	c.mqttClient.Subscribe(c.prefix+"/connected", 1, c.handleConnected)
	c.mqttClient.Subscribe(c.prefix+"/sections", 1, c.handleNumSections)
	c.mqttClient.Subscribe(c.prefix+"/sections/+", 1, c.handleSections)
}

func (c *GrinklersClient) handleConnected(mqttC mqtt.Client, message mqtt.Message) {
	connected, err := strconv.ParseBool(string(message.Payload()))
	if err != nil {
		c.chanConnected <- false
	} else {
		c.chanConnected <- connected
	}
}

func (c *GrinklersClient) handleNumSections(mqttC mqtt.Client, message mqtt.Message) {
	i, err := strconv.Atoi(string(message.Payload()))
	if err != nil {
		log.Errorf("invalid number received: %v", err)
	} else {
		c.chanNumSections <- i
	}
}
func (c *GrinklersClient) handleSections(mqttC mqtt.Client, message mqtt.Message) {
	var idx int
	topic := message.Topic()
	n, err := fmt.Sscanf(topic, c.prefix+"/sections/%d", &idx)
	if n != 1 || err != nil {
		log.WithError(err).WithField("topic", topic).Error("invalid section topic string")
	}
	sec := &Section{Id: idx}
	err = json.Unmarshal(message.Payload(), sec)
	if err != nil {
		log.WithError(err).Error("error in received section")
	} else {
		c.chanSections <- *sec
	}
}

func (c *GrinklersClient) IsConnected() bool {
	select {
	case connected := <-c.chanConnected:
		return connected
	case <-time.After(timeoutPeriod):
		return false
	}
}

func (c *GrinklersClient) getInteger(suffix string) (int, error) {
	cInt := make(chan interface{}, 1)
	path := c.prefix + suffix
	c.mqttClient.Subscribe(path, 1, func(c mqtt.Client, message mqtt.Message) {
		i, err := strconv.Atoi(string(message.Payload()))
		if err != nil {
			cInt <- fmt.Errorf("invalid number recieved: %v", err)
		}
		cInt <- i
	})
	defer c.mqttClient.Unsubscribe(path)
	select {
	case recv := <-cInt:
		i, ok := recv.(int)
		if !ok {
			return 0, recv.(error)
		}
		return i, nil
	case <-time.After(timeoutPeriod):
		return 0, timeoutError
	}
}

func (c *GrinklersClient) GetNumSections() (int, error) {
	if c.numSections != -1 {
		select {
		case numSections := <-c.chanNumSections:
			c.numSections = numSections
		default:
			break
		}
	} else {
		select {
		case numSections := <-c.chanNumSections:
			c.numSections = numSections
		case <-time.After(timeoutPeriod):
			return -1, timeoutError
		}
	}
	return c.numSections, nil
}

func (c *GrinklersClient) GetSections() ([]Section, error) {
	numSections, err := c.GetNumSections()
	if err != nil {
		return nil, err
	}
	var sections []Section
	timeoutChan := time.After(timeoutPeriod)
	recvSections := 0
	for recvSections < numSections {
		select {
		case newSec := <-c.chanSections:
			log.WithFields(log.Fields{
				"newSec": newSec,
			}).Debug("received sec")
			isNew := true
			for i, sec := range sections {
				if sec.Id == newSec.Id {
					sections[i] = newSec
					isNew = false
					break
				}
			}
			if isNew {
				sections = append(sections, newSec)
				recvSections++
			}
		case <-timeoutChan:
			return nil, timeoutError
		}
	}
	return sections, nil
}

func init() {
	level := os.Getenv("LOG_LEVEL")
	if level != "" {
		lvl, err := log.ParseLevel(level)
		if err == nil {
			log.SetLevel(lvl)
		}
	}
}

func createMqttOptions(mqttUrl *url.URL) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()

	brokerUrl := fmt.Sprintf("%s://%s", mqttUrl.Scheme, mqttUrl.Host)
	_, err := url.Parse(brokerUrl)
	if err != nil {
		log.WithError(err).Fatalln("invalid MQTT broker URL")
	}
	log.Debugf("broker url: '%v'", brokerUrl)
	opts.AddBroker(brokerUrl)

	opts.SetClientID(*clientID)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Info("connected to MQTT broker")
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.WithError(err).Info("disconnected from MQTT broker")
	})

	return opts
}

func main() {
	flag.Parse()
	var rawMqttUrl = flag.Arg(0)
	if rawMqttUrl == "" {
		rawMqttUrl = "tcp://localhost:1883"
	}

	mqttUrl, err := url.Parse(rawMqttUrl)
	if err != nil {
		log.WithError(err).Fatal("invalid MQTT URL")
	}

	prefix := mqttUrl.Path
	if prefix == "" {
		prefix = "grinklers"
	}
	log.WithFields(log.Fields{"mqttUrl": mqttUrl, "prefix": prefix}).Debugf("connecting to MQTT broker")

	mqttOpts := createMqttOptions(mqttUrl)
	mqttClient := mqtt.NewClient(mqttOpts)
	client := NewGrinklersClient(mqttClient, prefix)

	client.Connect()
	defer client.Disconnect()

	connected := client.IsConnected()
	entry := log.WithField("prefix", prefix)
	if connected {
		entry.Info("grinklers server is connected to broker")
	} else {
		entry.Fatalf("no grinklers server connected at prefix. exiting")
	}

	log.Debug("requesting number of sections")
	numSections, err := client.GetNumSections()
	if err != nil {
		log.WithError(err).Fatal("failed to retrieve number of sections")
	}
	log.WithField("numSections", numSections).Info("received number of sections")

	log.Debug("requesting sections")
	sections, err := client.GetSections()
	if err != nil {
		log.WithError(err).Fatal("failed to retrieve sections")
	}
	for _, section := range sections {
		log.Info(section)
	}
	log.WithField("sections", sections).Info("received sections")
}
