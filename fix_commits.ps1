$ErrorActionPreference = "Stop"

Write-Host "Resetting last commit..."
git reset HEAD~1

Write-Host "Committing M3..."
git add pkg/discord/commands/moderation/
release -m "refactor(commands): decompose moderation commands" -y --include-index pkg/discord/commands/moderation/ pkg/util/application.go

Write-Host "Committing Stylistic & Remaining..."
git add .
release -m "refactor(style): stylistic improvements and minor cleanups" -y
