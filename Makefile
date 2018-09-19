ifneq ($(wildcard ./.env),)
$(info using variables from .env file)
include ./.env
endif

GO               ?=go
RSYNC            ?=rsync
RSYNC_FLAGS      ?=-rv --partial --progress

SERVER_PACKAGE   :=./server
SERVER_BINARY    :=./grinklers
CLIENT_PACKAGE   :=./client
CLIENT_BINARY    :=./grinklers_client

GO_TOP			 :=git.amikhalev.com/amikhalev/grinklers
GO_PACKAGES      :=$(shell go list ./...)
GO_PACKAGE_PATHS :=$(subst $(GO_TOP),.,$(GO_PACKAGES))
GO_SOURCES       :=$(shell find . -type f -name '*.go' -not -name '*_test.go')
GO_TESTS         :=$(shell find . -type f -name '*_test.go')

$(info GO_PACKAGE_PATHS: $(GO_PACKAGE_PATHS))

STATIC_FILES     :=config.example.json grinklers.service

TEST_TIMEOUT     ?=10s
TEST_FLAGS       ?=-timeout $(TEST_TIMEOUT)
COV_OUTPUTS      :=$(addsuffix /coverage.out,$(GO_PACKAGE_PATHS))
COV_ALL          :=./coverage.all.out
COV_HTML         :=./coverage.html

DEPLOY_DIR       :=./rpi_deploy
DEPLOY_BINARY    :=$(DEPLOY_DIR)/grinklers
DEPLOY_FILES     :=$(addprefix $(DEPLOY_DIR)/,$(STATIC_FILES))
DEPLOY_ENV       ?=GOOS=linux GOARCH=arm GOARM=6
DEPLOY_HOST      ?=sprinklers@sprinklers.local
DEPLOY_PATH      ?=/opt/sprinklers

CLEAN_FILES      :=$(SERVER_BINARY) $(CLIENT_BINARY) $(DEPLOY_DIR) $(COV_OUTPUTS) $(COV_ALL) $(COV_HTML)

.PHONY: all
all: $(SERVER_BINARY) $(CLIENT_BINARY) vet test cover

.PHONY: clean
clean:
	$(GO) clean
	rm -rf $(CLEAN_FILES)

$(SERVER_BINARY): $(GO_SOURCES)
	$(GO) build -o $(SERVER_BINARY) $(SERVER_PACKAGE)

$(CLIENT_BINARY): $(GO_SOURCES)
	$(GO) build -o $(CLIENT_BINARY) $(CLIENT_PACKAGE)

.PHONY: get-deps
get-deps: $(GO_SOURCES)
	$(GO) get -t ./...

.PHONY: run
start: $(SERVER_BINARY)
	$(SERVER_BINARY)

.PHONY: client
client: $(CLIENT_BINARY)
	$(CLIENT_BINARY)

.PHONY: vet
vet: $(GO_SOURCES)
	$(GO) vet ./...

.PHONY: test
test: $(GO_SOURCES) $(GO_TESTS)
	$(GO) test $(TEST_FLAGS) ./...

$(COV_OUTPUTS): %: $(GO_SOURCES) $(GO_TESTS)
	$(GO) test ./$(@D) $(TEST_FLAGS) -coverprofile=$@
	@if [ ! -f $@ ]; then touch $@; fi

$(COV_ALL): $(COV_OUTPUTS)
	@echo "generating $@"
	@echo "mode: set" > $@
	@for out in $^ ; do cat $$out | grep -v "mode: set" >> $@; done || true

$(COV_HTML): $(COV_ALL)
	@echo "generating $@"
	@$(GO) tool cover -html=$(COV_ALL) -o $(COV_HTML)

.PHONY: cover
cover: $(COV_HTML)
	@echo "coverage generated. open file://$(realpath $<) in your web browser to view"

$(DEPLOY_DIR):
	mkdir -p $(DEPLOY_DIR)

$(DEPLOY_BINARY): $(GO_SOURCES) $(DEPLOY_DIR)
	$(DEPLOY_ENV) $(GO) build -o $(DEPLOY_BINARY) $(SERVER_PACKAGE)

$(DEPLOY_FILES): $(DEPLOY_DIR)/%: ./%
	cp $< $@

.PHONY: deploy
deploy: $(DEPLOY_DIR) $(DEPLOY_BINARY) $(DEPLOY_FILES)
	$(RSYNC) $(RSYNC_FLAGS) $(DEPLOY_DIR)/ $(DEPLOY_HOST):$(DEPLOY_PATH)/
	@echo "deploy successfully completed"
