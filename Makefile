SHELL := /bin/bash

PROVIDER_NAME := nameam
PKG := github.com/rserag/terraform-provider-nameam
BIN := terraform-provider-$(PROVIDER_NAME)
VERSION ?= 0.1.1
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
PLUGIN_DIR := $(HOME)/.terraform.d/plugins/registry.terraform.io/rserag/$(PROVIDER_NAME)/$(VERSION)/$(GOOS)_$(GOARCH)

.PHONY: all fmt vet test build install lint tidy

all: fmt vet test build

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./... -count=1

build:
	mkdir -p bin
	go build -o bin/$(BIN) .

install: build
	mkdir -p $(PLUGIN_DIR)
	cp bin/$(BIN) $(PLUGIN_DIR)/$(BIN)

tidy:
	go mod tidy

# Optional: run acceptance tests (requires env vars)
acc:
	TF_ACC=1 go test ./... -count=1 -run TestAcc
