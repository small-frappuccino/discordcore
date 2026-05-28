$ErrorActionPreference = "Stop"

Write-Host "Committing m7..."
release -m "refactor(config): move deprecated types to unmarshal structs" -y pkg/files/types.go pkg/files/preferences.go pkg/files/features.go pkg/util/application.go

Write-Host "Committing M3..."
git add pkg/discord/commands/moderation/
release -m "refactor(commands): decompose moderation commands" -y pkg/discord/commands/moderation/ pkg/util/application.go

Write-Host "Committing Stylistic & Remaining..."
git add .
release -m "refactor(style): stylistic improvements and minor cleanups" -y
