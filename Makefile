CC=go build
CFLAGS=
VERSION=2.5.6
LDFLAGS=-ldflags="-X 'jlortiz.org/jlort2/modules/commands.buildDate=$(shell date)' -X 'jlortiz.org/jlort2/modules/commands.verNum=$(VERSION)'"
FILES=$(wildcard *.go) $(wildcard modules/*/*.go)

.PHONY: all

all: jlort2

jlort2:  $(FILES)
	$(CC) $(CFLAGS) $(LDFLAGS)
