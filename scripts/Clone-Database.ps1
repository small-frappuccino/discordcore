param(
    [Parameter(Mandatory=$true)]
    [string]$NewDB,

    [Parameter(Mandatory=$true)]
    [string]$NewUser,

    [Parameter(Mandatory=$true)]
    [string]$NewPassword,

    [string]$SourceDB = "postgres",
    [string]$SourceUser = "postgres"
)

$ErrorActionPreference = "Stop"

Write-Host "Creating new user role '$NewUser'..."
# Create the user if it doesn't exist, otherwise update password.
# To handle existing users gracefully, we can just run CREATE ROLE and ignore errors, 
# or wrap it in a DO block.
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
# If DB exists, drop it first (be careful, but it's a clone script). Or just fail if exists.
psql -U $SourceUser -d postgres -c "CREATE DATABASE `"$NewDB`" OWNER `"$NewUser`";"

Write-Host "Dumping data from '$SourceDB' to '$NewDB' and excluding config data..."
# We use cmd.exe /c to avoid PowerShell's text encoding issues with pipes
cmd.exe /c "pg_dump -U $SourceUser -d $SourceDB --exclude-table-data=bot_config_state | psql -U $SourceUser -d $NewDB"

Write-Host "Database clone complete!"
Write-Host "New Database URL: postgres://$NewUser`:$NewPassword@127.0.0.1:5432/${NewDB}?sslmode=disable"
