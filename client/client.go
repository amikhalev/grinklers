package main

import (
	"flag"
	"net/url"
	"github.com/eclipse/paho.mqtt.golang"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"os"
	"strconv"
	"time"
	"errors"
)

var (
	clientId = flag.String("cid", "grinklers_client", "The MQTT client ID to connect with")
)

var (
	timeoutPeriod = 100 * time.Millisecond
	timeoutError = errors.New("the operation timed out")
)

type GrinklersClient struct {
	mqttClient mqtt.Client
	prefix string
}

func NewGrinklersClient(mqttClient mqtt.Client, prefix string) *GrinklersClient {
	return &GrinklersClient{mqttClient, prefix}
}

func (c *GrinklersClient) Connect() {
	if token := c.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.WithError(token.Error()).Fatal("error connecting to mqtt broker")
	}
}

func (c *GrinklersClient) Disconnect() {
	c.mqttClient.Disconnect(250)
}

func (c *GrinklersClient) IsConnected() bool {
	cConnected := make(chan bool, 1)
	path := c.prefix + "/connected"
	c.mqttClient.Subscribe(path, 1, func (c mqtt.Client, message mqtt.Message) {
		connected, err := strconv.ParseBool(string(message.Payload()))
		if err != nil {
			cConnected <- false
		} else {
			cConnected <- connected
		}
	})
	select {
	case connected := <-cConnected:
		return connected
	case <-time.After(timeoutPeriod):
		return false
	}
}

func (c *GrinklersClient) GetNumSections() (int, error) {
	cNumSections := make(chan interface{}, 1)
	path := c.prefix + "/sections"
	c.mqttClient.Subscribe(path, 1, func(c mqtt.Client, message mqtt.Message) {
		numSections, err := strconv.Atoi(string(message.Payload()))
		if err != nil {
			cNumSections <- fmt.Errorf("invalid number of sections recieved: %v", err)
		}
		cNumSections <- numSections
	})
	select {
	case recv := <-cNumSections:
		c.mqttClient.Unsubscribe(path)
		numSections, ok := recv.(int)
		if !ok {
			return 0, recv.(error)
		} else {
			return numSections, nil
		}
	case <-time.After(timeoutPeriod):
		return 0, timeoutError
	}
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

func createMqttOptions(mqttUrl *url.URL) (*mqtt.ClientOptions) {
	opts := mqtt.NewClientOptions()

	brokerUrl := fmt.Sprintf("%s://%s", mqttUrl.Scheme, mqttUrl.Host)
	_, err := url.Parse(brokerUrl)
	if err != nil {
		log.WithError(err).Fatalln("invalid MQTT broker URL")
	}
	log.Debugf("broker url: '%v'", brokerUrl)
	opts.AddBroker(brokerUrl)

	opts.SetClientID(*clientId)
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
	log.WithField("numSections", numSections).Info("recieved number of sections")
}
