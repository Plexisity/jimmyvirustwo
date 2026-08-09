#!/usr/bin/env bash

GOOS=windows GOARCH=amd64 go build  -o update.exe update.go

# -ldflags "-H windowsgui"