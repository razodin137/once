#!/usr/bin/env bash
# Run the split prompts in prompt/ one at a time, each in a
# fresh session, stopping at the first failure.
#
#   MODEL=ollama/deepseek-v4.1-flash:1017-cloud ./scripts/split-serial.sh
#   MODEL=... FROM=8 TO=8 ./scripts/split-serial.sh    # one step only
#   ONLY=11-state,07-remove,08-shell ./scripts/split-serial.sh   # explicit order
#   COMMIT=0 ./scripts/split-serial.sh                 # leave commits alone
#   CHECK_CMD='make test' ./scripts/split-serial.sh    # override the detected gate
#
# No --continue and no --session: that is what makes every step a fresh agent.
# The tree must be clean before the loop starts, and every green step is
# committed, so a bad step is one `git revert` away.

set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
LOGS="$ROOT/.scratch/split"
MODEL=${MODEL:-ollama/deepseek-v4.1-flash:1017-cloud}
FROM=${FROM:-1}
TO=${TO:-99}
COMMIT=${COMMIT:-1}
ONLY=${ONLY:-}
CHECK_CMD=${CHECK_CMD:-__detect__}

mkdir -p "$LOGS"
cd "$ROOT" || exit 1

has_make_target() {
  make -n "$1" >/dev/null 2>&1
}

detect_check() {
  if [ "$CHECK_CMD" != "__detect__" ]; then
    printf '%s\n' "$CHECK_CMD"
  elif [ -f package.json ] && node -e \
      'process.exit(require("./package.json").scripts?.check ? 0 : 1)' 2>/dev/null; then
    printf 'npm run check\n'
  elif [ -f Makefile ]; then
    local check="" target
    for target in test lint; do
      if has_make_target "$target"; then
        check="${check:+$check && }make $target"
      fi
    done
    if [ -z "$check" ] && has_make_target check; then
      check="make check"
    fi
    printf '%s\n' "$check"
  fi
}

CHECK=$(detect_check)

if [ -z "$CHECK" ]; then
  echo "== no project check detected; set CHECK_CMD to gate each step" >&2
else
  echo "== check: $CHECK"
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "== tree is dirty; commit it before starting" >&2
  exit 1
fi

if [ -n "$ONLY" ]; then
  IFS=',' read -r -a files <<<"$ONLY"
  for i in "${!files[@]}"; do
    files[$i]="$ROOT/prompt/${files[$i]}.md"
  done
else
  files=("$ROOT"/prompt/*.md)
fi

for path in "${files[@]}"; do
  name=$(basename "$path" .md)
  step=$((10#${name%%-*}))

  if [ -z "$ONLY" ]; then
    [ "$step" -lt "$FROM" ] && continue
    [ "$step" -gt "$TO" ] && continue
  fi

  if [ ! -f "$path" ]; then
    echo "== $name: no such command" >&2
    exit 1
  fi

  blocked="$LOGS/BLOCKED-$name"
  rm -f "$blocked"

  echo "== $name"
  opencode run -m "$MODEL" --auto --title "split $name" "/split/$name" 2>&1 \
    | tee "$LOGS/$name.log"
  status=${PIPESTATUS[0]}

  if [ -f "$blocked" ]; then
    echo "== $name: blocked, $(cat "$blocked")" >&2
    exit 1
  fi

  if [ "$status" -ne 0 ]; then
    echo "== $name: opencode exited $status, stopping" >&2
    exit 1
  fi

  if [ -n "$CHECK" ] && ! eval "$CHECK" >"$LOGS/$name.check" 2>&1; then
    echo "== $name: check failed ($CHECK), see $LOGS/$name.check" >&2
    exit 1
  fi

  git diff --stat >"$LOGS/$name.diff"

  if [ "$COMMIT" = "1" ]; then
    git add -A
    git commit -q -m "split: $name"
  fi
done

echo "== finished"
