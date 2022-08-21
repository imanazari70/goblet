#!/usr/bin/env bash
set -euo pipefail

cd /app
go build -o /tmp/packobjectshook ./hooks/packobjects
