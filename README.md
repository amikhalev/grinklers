[![Build Status](https://travis-ci.org/amikhalev/grinklers.svg?branch=master)](https://travis-ci.org/amikhalev/grinklers)

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
    
To deploy to a Raspberry PI:

```shell
DEPLOY_HOST=<host> DEPLOY_USER=<user> DEPLOY_PATH=<path> make deploy
```

Environment variables will be loaded from a `.env` file in the root
directory of the project.