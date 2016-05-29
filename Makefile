ifneq ("$(wildcard $(.env))","")
include .env
endif

BINARY     = ./grinklers
COV_OUTPUT = ./coverage.out
COV_HTML   = ./coverage.html

GO := go

GO_SOURCES   := $(shell find . -type f -name '*.go' -not -name '*_test.go')
GO_TESTS     := $(shell find . -type f -name '*_test.go')
STATIC_FILES  = config.json

DEPLOY_DIR    = ./rpi_deploy
DEPLOY_BINARY = $(DEPLOY_DIR)/grinklers
DEPLOY_ENV    = GOOS=linux GOARCH=arm GOARM=6
DEPLOY_FILES  = $(addprefix $(DEPLOY_DIR)/,$(STATIC_FILES))
DEPLOY_HOST  ?= 192.168.1.30
DEPLOY_USER  ?= alex
DEPLOY_PATH  ?= /home/alex/grinklers

CLEAN_FILES = $(DEPLOY_DIR) $(COV_OUTPUT) $(COV_HTML)

.PHONY: all clean deps run test cover deploy

all: run

clean:
	$(GO) clean
	rm -rf $(CLEAN_FILES)

$(BINARY): $(GO_SOURCES)
	$(GO) build -o ${BINARY} ./main

deps: $(GO_SOURCES)
	$(GO) get -t ./...

run: $(BINARY)
	$(BINARY)

test: $(GO_SOURCES) $(GO_TESTS)
	$(GO) test

$(COV_OUTPUT): $(GO_SOURCES) $(GO_TESTS)
	$(GO) test -coverprofile=${COV_OUTPUT} ./...

$(COV_HTML): $(COV_OUTPUT)
	$(GO) tool cover -html=$(COV_OUTPUT) -o $(COV_HTML)

cover: $(COV_HTML)
#	start $(COV_HTML) || xdg-open $(COV_HTML) || open $(COV_HTML)

$(DEPLOY_DIR):
	mkdir -p $(DEPLOY_DIR)

$(DEPLOY_BINARY): $(GO_SOURCES) $(DEPLOY_DIR)
	$(DEPLOY_ENV) $(GO) build -o $(DEPLOY_BINARY)

$(DEPLOY_FILES): $(DEPLOY_DIR)/%: ./%
	cp $< $@

deploy: $(DEPLOY_DIR) $(DEPLOY_BINARY) $(DEPLOY_FILES)
	scp $(DEPLOY_DIR)/ $(DEPLOY_USER)@$(DEPLOY_HOST):$(DEPLOY_PATH)
