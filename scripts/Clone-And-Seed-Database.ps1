param(
    [Parameter(Mandatory=$true)]
    [string]$NewDB,

    [Parameter(Mandatory=$true)]
    [string]$NewUser,

    [Parameter(Mandatory=$true)]
    [string]$NewPassword,

    [Parameter(Mandatory=$true)]
    [string]$MainToken,

    [Parameter(Mandatory=$true)]
    [string]$QOTDToken,

    [string]$SourceDB = "postgres",
    [string]$SourceUser = "postgres"
)

$ErrorActionPreference = "Stop"

Write-Host "Creating new user role '$NewUser'..."
$createUserSql = "
DO `$`$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '$NewUser') THEN
        CREATE ROLE `"$NewUser`" WITH LOGIN PASSWORD '$NewPassword';
    ELSE
        ALTER ROLE `"$NewUser`" WITH PASSWORD '$NewPassword';
    END IF;
END
`$`$;
"
psql -U $SourceUser -d postgres -c $createUserSql

Write-Host "Creating new database '$NewDB' owned by '$NewUser'..."
psql -U $SourceUser -d postgres -c "CREATE DATABASE `"$NewDB`" OWNER `"$NewUser`";"

Write-Host "Dumping data from '$SourceDB' to '$NewDB' and excluding config data..."
# Run pg_dump with --no-owner so the new tables belong to the new db owner when imported.
# We set PGPASSWORD so we can pipe into psql using the new user!
$env:PGPASSWORD = $NewPassword
cmd.exe /c "pg_dump -U $SourceUser -d $SourceDB --exclude-table-data=bot_config_state --no-owner | psql -U $NewUser -d $NewDB"
$env:PGPASSWORD = ""

Write-Host "Seeding required guild config into new database..."
$newDbUrl = "postgres://$NewUser`:$NewPassword@127.0.0.1:5432/${NewDB}?sslmode=disable"
go run scripts/Seed-GuildConfig.go -db "$newDbUrl"

Write-Host "Saving bot tokens to environment file..."
$envPath = "D:\Users\alice\.local\bin\.env"
Add-Content -Path $envPath -Value "`nALICE_BOT_PRODUCTION_TOKEN=$MainToken"
Add-Content -Path $envPath -Value "QOTD_BOT_PRODUCTION_TOKEN=$QOTDToken"

Write-Host "Database clone complete!"
Write-Host "New Database URL: $newDbUrl"
