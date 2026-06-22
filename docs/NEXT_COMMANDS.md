# Daily Commands

## Project root

```powershell
cd D:\Projects\freight-platform
```

## Check current state

```powershell
git status --short
git log --oneline -5
```

## Start backend

```powershell
make platform-up-no-build
make health-check
```

## Seed dev data

```powershell
make seed-dev-admin
make seed-demo-data
```

## Run smoke test

```powershell
make integration-smoke-test
```

## Start frontend

```powershell
cd D:\Projects\freight-platform\apps\web-admin
npm run dev
```

Open:

```text
http://localhost:3000/login
```

Login:

```text
Tenant ID: 74519f22-ff9b-4a8b-8fff-a958c689682f
Email: admin@7rights.local
Password: Admin123456!
```

## Commit

```powershell
cd D:\Projects\freight-platform
git status --short
git add .
git commit -m "..."
git push origin main
```

## Last commits

```powershell
git log --oneline -5
```
