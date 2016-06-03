[![Build Status](https://travis-ci.org/amikhalev/grinklers.svg?branch=master)](https://travis-ci.org/amikhalev/grinklers) [![Coverage Status](https://coveralls.io/repos/github/amikhalev/grinklers/badge.svg?branch=master)](https://coveralls.io/github/amikhalev/grinklers?branch=master)

Grinklers
=========

Sprinklers system controller written in Go.

Features:
---------
 
 * Configurable with JSON
 * Communicates to an MQTT broker for remote operation
 * Sections ran through RPi GPIO pins
 * Programs support flexible run times for each section
 * Flexible scheduling for programs

Usage:
------

To run locally:

```shell
make run
```
    
To run tests:

```shell
make test
```
    
To get test coverage:

```shell
make cover
```
    
To deploy to a Raspberry PI (note: any files in `./rpi_deploy` will be
deployed, so beware of overriding configuration files):

```shell
DEPLOY_HOST=<user@host:port> DEPLOY_PATH=<path> make deploy
```

Environment variables will be loaded from a `.env` file in the root
directory of the project.