ifeq ($(OS),Windows_NT)
	DATE=$(shell powershell -noprofile get-date -format "{ddd dd MMM yyyy hh:mm:ss tt K}")
	TARGET=jlort2.exe
else
	DATE=$(shell date)
	TARGET=jlort2
endif

CC=go build
CFLAGS=
VERSION=3.2.5
LDFLAGS=-ldflags="-X 'jlortiz.org/jlort2/modules/commands.buildDate=$(DATE)' -X 'jlortiz.org/jlort2/modules/commands.verNum=$(VERSION)' -s -w"
FILES=$(wildcard *.go) $(wildcard modules/*/*.go)

.PHONY: all

all: $(TARGET)

$(TARGET): $(FILES)
	$(CC) $(CFLAGS) $(LDFLAGS)
