# Format gate for discordcore. Fails non-zero on any of:
#   - Go files that gofmt would rewrite (formatting OR CRLF line endings)
#   - Tracked files committed with line endings that disagree with .gitattributes
# Run locally before opening a PR. Exit 0 when the index is clean.

$ErrorActionPreference = 'Stop'
$failed = $false

# 1. Gofmt drift. -l prints files that would be rewritten by gofmt -w.
$gofmtDirty = & gofmt -l .
if ($LASTEXITCODE -ne 0) {
    Write-Error "gofmt invocation failed (exit $LASTEXITCODE)"
    exit 1
}
if ($gofmtDirty) {
    Write-Host "gofmt drift in:" -ForegroundColor Red
    $gofmtDirty | ForEach-Object { Write-Host "  $_" }
    Write-Host "Fix: gofmt -w ." -ForegroundColor Yellow
    $failed = $true
}

# 2. Index line-ending drift against .gitattributes. git ls-files --eol
# emits one row per tracked file with the index encoding, working-tree
# encoding, and the attribute decision. We only police the index since
# the working tree may legitimately differ on Windows with
# core.autocrlf=true. A file is flagged when the attribute says eol=lf
# but the committed blob in the index is anything else, or when the
# attribute says eol=crlf and the committed blob is lf.
$eolRows = & git ls-files --eol
$eolDrift = @()
foreach ($row in $eolRows) {
    $columns = $row -split '\s+', 4
    if ($columns.Length -lt 4) { continue }
    $indexEol = $columns[0]
    $attr = $columns[2]
    $path = $columns[3]
    if ($attr -match 'eol=lf' -and $indexEol -ne 'i/lf') {
        $eolDrift += "  expected lf:   $path"
    }
    elseif ($attr -match 'eol=crlf' -and $indexEol -ne 'i/crlf') {
        $eolDrift += "  expected crlf: $path"
    }
}
if ($eolDrift.Count -gt 0) {
    Write-Host "Line-ending drift against .gitattributes:" -ForegroundColor Red
    $eolDrift | ForEach-Object { Write-Host $_ }
    Write-Host "Fix: git add --renormalize ." -ForegroundColor Yellow
    $failed = $true
}

if ($failed) {
    exit 1
}

Write-Host "Format gate: clean." -ForegroundColor Green
exit 0
