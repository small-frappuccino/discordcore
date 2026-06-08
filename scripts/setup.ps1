param ()


Write-Host "Installing UI dependencies..."
cd ui
bun install
bun add -d eslint typescript
cd ..

Write-Host "Setup completed successfully!"
