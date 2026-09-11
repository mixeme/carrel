@echo off
cd /d "%~dp0..\typecase"
if "%PORT%"=="" set "PORT=8081"
go run ./cmd/screens
