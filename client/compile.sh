#!/usr/bin/env bash

GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o main.exe main.go
