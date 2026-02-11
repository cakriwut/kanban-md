#!/bin/bash
# One-command TUI demo GIF generator.
# Usage: bash assets/tui-demo-gen.sh
# Requirements: asciinema, agg, python3
set -e

echo "Building kanban-md..."
go build -o /tmp/kanban-md ./cmd/kanban-md

echo "Recording TUI demo..."
asciinema rec assets/tui-demo.cast \
  --window-size 140x30 \
  --command "bash assets/tui-demo-record.sh" --overwrite

echo "Capping OSC query delays..."
python3 -c "
import json
lines = open('assets/tui-demo.cast').readlines()
out = open('assets/tui-demo-capped.cast', 'w')
for line in lines:
    try:
        ev = json.loads(line)
        if isinstance(ev, list) and ev[0] > 5.0:
            ev[0] = 0.5
        out.write(json.dumps(ev) + '\n')
    except (json.JSONDecodeError, TypeError):
        out.write(line)
out.close()
"
mv assets/tui-demo-capped.cast assets/tui-demo.cast

echo "Converting to GIF..."
agg assets/tui-demo.cast assets/tui-demo.gif \
  --font-size 16 --theme dracula --idle-time-limit 3 --last-frame-duration 3

echo "Wrote assets/tui-demo.gif"
