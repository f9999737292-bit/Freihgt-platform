# Safe, lightweight worktree diagnostics for Bintrans Freight Platform (Windows).
# Intentionally does NOT install dependencies, copy secrets, start Docker, or run migrations.

$ErrorActionPreference = 'Stop'

Write-Host '=== Bintrans worktree setup (diagnostics only) ==='

$worktreePath = (git rev-parse --show-toplevel 2>$null)
if (-not $worktreePath) {
    Write-Error 'Not inside a Git worktree.'
}

Write-Host "Worktree path: $worktreePath"

if ($env:ROOT_WORKTREE_PATH) {
    Write-Host "Root worktree path: $env:ROOT_WORKTREE_PATH"
} else {
    Write-Host 'Root worktree path: (ROOT_WORKTREE_PATH not set)'
}

$branch = git branch --show-current 2>$null
Write-Host "Git branch: $branch"

Write-Host 'Git status:'
git status --short

$markers = @(
    'Makefile',
    'go.work',
    'pnpm-workspace.yaml',
    'AGENTS.md',
    'infrastructure/migrations',
    'packages/openapi/openapi.yaml'
)

Write-Host 'Repository markers:'
foreach ($marker in $markers) {
    $fullPath = Join-Path $worktreePath $marker
    if (Test-Path $fullPath) {
        Write-Host "  OK  $marker"
    } else {
        Write-Host "  MISSING  $marker"
    }
}

Write-Host 'Worktree setup complete (diagnostics only).'
Write-Host 'Future optional steps (NOT run automatically): pnpm install, go mod download, copy .env from root worktree, make dev-up, make migrate-up.'
