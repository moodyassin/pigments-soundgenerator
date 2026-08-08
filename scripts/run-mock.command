#!/bin/bash
set -e
cd "$(dirname "$0")/.."
go run . serve --mock --open
