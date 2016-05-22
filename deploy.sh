#!/bin/sh
GOOS=linux GOARCH=arm GOARM=6 go build
scp ./grinklers ./config.json alex@192.168.1.30:/home/alex/grinklers/

# MQTT_BROKER=tcp://gtvmloxb:6P8-7qngI7DJ@m12.cloudmqtt.com:16737