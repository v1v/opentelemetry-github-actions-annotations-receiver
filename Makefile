SHELL := bash
MAKEFLAGS += --no-print-directory
WEBHOOK_SECRET ?= secret
GITHUB_TOKEN ?= $(shell gh auth token)

#######################
## Tools
#######################
export PATH := $(CURDIR)/bin:$(PATH)
OCB ?= $(CURDIR)/bin/builder

## @help:install-ngrok:Install ngrok.
.PHONY: install-ngrok
install-ngrok:
ifeq ($(OS),Darwin)
	brew install ngrok/ngrok/ngrok
else
	$(error "Please install ngrok manually")
endif

## @help:install-ocb:Install ocb.
.PHONY: install-ocb
install-ocb:
	GOBIN=$(CURDIR)/bin go install go.opentelemetry.io/collector/cmd/builder@v0.102.0

## MAKE GOALS
.PHONY: build
build: ## Build the binary
	@$(OCB) --config builder-config.yml

.PHONY: run
run: ## Run the binary
	@WEBHOOK_SECRET=$(WEBHOOK_SECRET) \
	GITHUB_TOKEN=$(GITHUB_TOKEN) \
	./bin/otelcol-custom --config config.yml

.PHONY: ngrok
ngrok: ## Run ngrok
	ngrok http http://localhost:33333
