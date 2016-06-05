ifneq ($(wildcard ./.env),)
$(info using variables from .env file)
include ./.env
endif

BINARY     = ./grinklers

GO := go

GO_PACKAGES      := $(shell go list ./...)
GO_PACKAGE_PATHS := $(subst $(word 1,$(GO_PACKAGES)),./,$(GO_PACKAGES))
GO_SOURCES       := $(shell find . -type f -name '*.go' -not -name '*_test.go')
GO_TESTS         := $(shell find . -type f -name '*_test.go')

STATIC_FILES = config.example.json

TEST_TIMEOUT = 10s
COV_OUTPUT  := coverage.out
COV_OUTPUTS := $(addsuffix /$(COV_OUTPUT),$(GO_PACKAGE_PATHS))
COV_ALL     := ./coverage.all.out
COV_HTML    := ./coverage.html

DEPLOY_DIR    = ./rpi_deploy
DEPLOY_BINARY = $(DEPLOY_DIR)/grinklers
DEPLOY_ENV    = GOOS=linux GOARCH=arm GOARM=6
DEPLOY_FILES  = $(addprefix $(DEPLOY_DIR)/,$(STATIC_FILES))
DEPLOY_HOST  ?= alex@192.168.1.30
DEPLOY_PATH  ?= /home/alex/grinklers

CLEAN_FILES = $(BINARY) $(DEPLOY_DIR) $(COV_OUTPUTS) $(COV_ALL) $(COV_HTML)

.PHONY: all clean deps run test cover deploy

all: run

clean:
	$(GO) clean
	rm -rf $(CLEAN_FILES)

$(BINARY): $(GO_SOURCES)
	$(GO) build -o ${BINARY} ./main

deps: $(GO_SOURCES)
	$(GO) get -t $(GO_PACKAGES)

run: $(BINARY)
	$(BINARY)

test: $(GO_SOURCES) $(GO_TESTS)
	$(GO) test -timeout $(TEST_TIMEOUT) $(GO_PACKAGES)

$(COV_OUTPUTS): %: $(GO_SOURCES) $(GO_TESTS)
	$(GO) test ./$(@D) -timeout $(TEST_TIMEOUT) -coverprofile=$@
	@if [ ! -f $@ ]; then touch $@; fi

$(COV_ALL): $(COV_OUTPUTS)
	@echo "generating $@"
	@echo "mode: set" > $@
	@for out in $^ ; do cat $$out | grep -v "mode: set" >> $@; done

$(COV_HTML): $(COV_ALL)
	@echo "generating $@"
	@$(GO) tool cover -html=$(COV_ALL) -o $(COV_HTML)

cover: $(COV_HTML)
	@echo "coverage generated. open file://$(realpath $<) in your web browser to view"

$(DEPLOY_DIR):
	mkdir -p $(DEPLOY_DIR)

$(DEPLOY_BINARY): $(GO_SOURCES) $(DEPLOY_DIR)
	$(DEPLOY_ENV) $(GO) build -o $(DEPLOY_BINARY) ./main

$(DEPLOY_FILES): $(DEPLOY_DIR)/%: ./%
	cp $< $@

deploy: $(DEPLOY_DIR) $(DEPLOY_BINARY) $(DEPLOY_FILES)
	scp -r $(DEPLOY_DIR)/* $(DEPLOY_HOST):$(DEPLOY_PATH)/
	@echo "deploy successfully completed"
