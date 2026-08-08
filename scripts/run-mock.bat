@echo off
cd /d "%~dp0\.."
go run . serve --mock --open
if errorlevel 1 pause
