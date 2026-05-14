#!/usr/bin/env bash
# Format gate for discordcore. Fails non-zero on any of:
#   - Go files that gofmt would rewrite (formatting OR CRLF line endings)
#   - Tracked files committed with line endings that disagree with .gitattributes
# Run locally before opening a PR. Exit 0 when the index is clean.

set -u

failed=0

# 1. Gofmt drift. -l prints files that would be rewritten by gofmt -w.
gofmt_dirty=$(gofmt -l .)
if [ -n "$gofmt_dirty" ]; then
    echo "gofmt drift in:" >&2
    printf '  %s\n' $gofmt_dirty >&2
    echo "Fix: gofmt -w ." >&2
    failed=1
fi

# 2. Index line-ending drift against .gitattributes. git ls-files --eol
# emits one row per tracked file with the index encoding, working-tree
# encoding, and the attribute decision. We only police the index since
# the working tree may legitimately differ on Windows with
# core.autocrlf=true. A file is flagged when the attribute says eol=lf
# but the committed blob in the index is anything else (mixed or crlf),
# or when the attribute says eol=crlf and the committed blob is lf.
eol_drift=$(git ls-files --eol \
    | awk '
        $3 ~ /eol=lf/   && $1 != "i/lf"   { print "  expected lf:   " $4 }
        $3 ~ /eol=crlf/ && $1 != "i/crlf" { print "  expected crlf: " $4 }
    ')
if [ -n "$eol_drift" ]; then
    echo "Line-ending drift against .gitattributes:" >&2
    printf '%s\n' "$eol_drift" >&2
    echo "Fix: git add --renormalize ." >&2
    failed=1
fi

if [ "$failed" -ne 0 ]; then
    exit 1
fi

echo "Format gate: clean."
