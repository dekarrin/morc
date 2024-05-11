#!/bin/bash

env CGO_ENABLED=0 go build -o morc cmd/morc/main.go
