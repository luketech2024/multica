param(
    [string]$GoCommand = 'go'
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    $cliVersion = git describe --tags --match 'v[0-9]*' --dirty
    if ($LASTEXITCODE -ne 0 -or $cliVersion -notmatch '^v\d+\.\d+\.\d+') {
        throw 'No release tag is available. Fetch release tags before building the CLI.'
    }
    $buildCommit = git rev-parse HEAD
    if ($LASTEXITCODE -ne 0) { throw 'Cannot determine the source commit.' }
    $shortCommit = git rev-parse --short HEAD
    if ($LASTEXITCODE -ne 0) { throw 'Cannot determine the short source commit.' }
    $buildDate = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
    $linkFlags = "-X main.version=$cliVersion -X main.commit=$shortCommit -X main.buildCommit=$buildCommit -X main.date=$buildDate"
    Set-Location (Join-Path $repoRoot 'server')
    & $GoCommand build -ldflags $linkFlags -o bin/multica-local.exe ./cmd/multica
    if ($LASTEXITCODE -ne 0) { throw 'CLI build failed.' }
    & ./bin/multica-local.exe --version
    if ($LASTEXITCODE -ne 0) { throw 'CLI version verification failed.' }
} finally {
    Pop-Location
}
