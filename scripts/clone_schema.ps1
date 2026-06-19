param (
    [string]$SourceDb = "alicemains",
    [string]$PgAdminUser = "postgres"
)

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "   Clonagem de Banco de Dados (Somente Esquema) " -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

$TargetDb = Read-Host "Digite o nome para o novo banco de dados (ex: alicemains_dev)"
$TargetUser = Read-Host "Digite o nome do novo usuário/role do banco"
$TargetPassword = Read-Host "Digite a senha para o novo usuário"

if ([string]::IsNullOrWhiteSpace($TargetDb) -or [string]::IsNullOrWhiteSpace($TargetUser)) {
    Write-Error "Nome do banco e usuário são obrigatórios."
    exit 1
}

$DumpFile = "schema_dump_temp_$([guid]::NewGuid().Guid).sql"

try {
    Write-Host "`n[1/4] Extraindo a estrutura (esquema) do banco de dados '$SourceDb' (sem os dados)..." -ForegroundColor Yellow
    # O parâmetro -s (--schema-only) garante que apenas a estrutura seja exportada
    pg_dump -U $PgAdminUser -d $SourceDb -s -f $DumpFile
    if ($LASTEXITCODE -ne 0) {
        throw "Falha ao exportar o esquema usando pg_dump."
    }

    Write-Host "[2/4] Criando o usuário '$TargetUser'..." -ForegroundColor Yellow
    $CreateUserQuery = "CREATE USER $TargetUser WITH PASSWORD '$TargetPassword';"
    psql -U $PgAdminUser -c $CreateUserQuery -d postgres
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "O usuário pode já existir. Continuando..."
    }

    Write-Host "[3/4] Criando o novo banco de dados '$TargetDb' e atribuindo ao usuário '$TargetUser'..." -ForegroundColor Yellow
    $CreateDbQuery = "CREATE DATABASE $TargetDb OWNER $TargetUser;"
    psql -U $PgAdminUser -c $CreateDbQuery -d postgres
    if ($LASTEXITCODE -ne 0) {
        throw "Falha ao criar o novo banco de dados."
    }

    Write-Host "[4/4] Importando a estrutura para o novo banco de dados..." -ForegroundColor Yellow
    psql -U $PgAdminUser -d $TargetDb -f $DumpFile
    if ($LASTEXITCODE -ne 0) {
        throw "Falha ao importar o esquema para o novo banco."
    }

    Write-Host "`n==========================================" -ForegroundColor Green
    Write-Host "   Concluído com sucesso!                 " -ForegroundColor Green
    Write-Host "==========================================" -ForegroundColor Green
    Write-Host "Seu novo ambiente de testes de estresse está pronto."
    Write-Host "Banco: $TargetDb"
    Write-Host "Usuário: $TargetUser"

} catch {
    Write-Error $_.Exception.Message
} finally {
    if (Test-Path $DumpFile) {
        Write-Host "Limpando arquivos temporários..." -ForegroundColor DarkGray
        Remove-Item $DumpFile -ErrorAction SilentlyContinue
    }
}
