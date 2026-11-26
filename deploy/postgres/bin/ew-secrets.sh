#!/bin/sh
# Sourced helper: load_secret VAR FILE exports the content of a Docker secret file into the process environment
# only (never the service spec). A missing or empty file stops the process naming the variable (F130-AC-11).
load_secret() {
  if [ ! -s "$2" ]; then echo "$(basename "$0"): $1: secret file missing or empty" >&2; exit 1; fi
  eval "export $1=\"\$(cat \"\$2\")\""
}
load_secret_optional() { if [ -e "$2" ]; then load_secret "$1" "$2"; fi; }
