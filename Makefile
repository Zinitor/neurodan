PYTHON := .venv/bin/python
ANECDOTES_DIR := data/anecdotes
CORPUS := $(ANECDOTES_DIR)/anecdotes_clean.txt
INDEX := $(ANECDOTES_DIR)/anecdotes.idx
MODEL := models/model.onnx
GENERATOR_BIN := bin/generator
BOT_BIN := bin/bot
GO_SRC := $(wildcard cmd/*.go cmd/*/*.go internal/*/*.go) go.mod go.sum
PORT ?= 8000

.DEFAULT_GOAL := generator
.PHONY: generator bot build test lint clean

generator: $(GENERATOR_BIN) $(CORPUS) $(MODEL)
	ANECDOTES_PATH=$(CORPUS) \
	ANECDOTES_INDEX=$(INDEX) \
	PORT=$(PORT) \
	$(GENERATOR_BIN)

bot: $(BOT_BIN)
	$(BOT_BIN)

build: $(GENERATOR_BIN) $(BOT_BIN)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin

$(GENERATOR_BIN): $(GO_SRC)
	go build -o $@ ./cmd/generator

$(BOT_BIN): $(GO_SRC)
	go build -o $@ ./cmd/bot

$(PYTHON):
	uv venv
	uv pip install "sentence-transformers[onnx]" optimum onnx onnxscript

$(CORPUS): scripts/download_anecdotes.py scripts/filter_anecdotes.py | $(ANECDOTES_DIR)
	$(PYTHON) scripts/download_anecdotes.py
	$(PYTHON) scripts/filter_anecdotes.py

$(MODEL): scripts/export_model.py $(PYTHON)
	$(PYTHON) scripts/export_model.py

$(ANECDOTES_DIR):
	mkdir -p $(ANECDOTES_DIR)
