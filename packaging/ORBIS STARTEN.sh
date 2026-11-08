#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
chmod +x "$ROOT/Orbis"
exec "$ROOT/Orbis" ui --home "$ROOT"
