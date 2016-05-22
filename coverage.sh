#!/bin/sh

go test -coverprofile=coverage.out && sleep .5 && go tool cover -html=coverage.out