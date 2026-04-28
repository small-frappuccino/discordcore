param(
	[string]$DatabaseUrl = $env:DISCORDCORE_TEST_DATABASE_URL
)

if ([string]::IsNullOrWhiteSpace($DatabaseUrl)) {
	Write-Error "DISCORDCORE_TEST_DATABASE_URL is required for QOTD race tests."
	exit 1
}

$env:DISCORDCORE_TEST_DATABASE_URL = $DatabaseUrl
if ([string]::IsNullOrWhiteSpace($env:CGO_ENABLED)) {
	$env:CGO_ENABLED = "1"
}

$commands = @(
	@("go", "test", "-race", "./pkg/files", "./pkg/discord/commands/config"),
	@("go", "test", "-race", "-tags=integration", "./pkg/qotd"),
	@("go", "test", "-race", "-tags=integration", "./pkg/discord/commands/qotd"),
	@("go", "test", "-race", "-tags=integration", "./pkg/control", "-run", "QOTD"),
	@("go", "test", "-race", "-tags=integration", "./pkg/storage", "-run", "QOTD"),
	@("go", "test", "-race", "-tags=integration", "./pkg/persistence", "-run", "QOTD")
)

foreach ($command in $commands) {
	Write-Host ("> " + ($command -join " "))
	& $command[0] $command[1..($command.Length - 1)]
	if ($LASTEXITCODE -ne 0) {
		exit $LASTEXITCODE
	}
}