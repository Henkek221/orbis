#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Henrik Hellpap. Part of Orbis: https://github.com/Henkek221/orbis
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
chmod +x "$ROOT/Orbis"
exec "$ROOT/Orbis" ui --home "$ROOT"
