#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"
postcss ./jass.css -o jass.min.css