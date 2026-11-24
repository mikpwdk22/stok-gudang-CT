# CTStock — Backend & Database Foundation (Phase 1)

CTStock is a family inventory management system designed for store operations. This repository contains the Go backend and PostgreSQL database foundation.

> **Note for Collaborators:** The React frontend is located in the `frontend/` directory and is managed by another developer.

---

## 1. Technology Stack

- **Language:** Go (v1.27+)
- **Database:** PostgreSQL 16
- **Database Driver:** `pgx/v5` (`github.com/jackc/pgx/v5`) with connection pooling (`pgxpool`)
- **Containerization:** Docker Compose
- **Migrations:** SQL file-based migration runner (`backend/migrations/`)

---

## 2. Directory Structure

```text
ctstock/
├── docker-compose.yml          # PostgreSQL 16 container definition
├── .gitignore                  # Git exclusions (credentials, binaries, node_modules)
├── README.md                   # This documentation
│
├── backend/
│   ├── cmd/
│   │   └── api/
│   │       └── main.go         # REST API entry point & graceful shutdown
│   ├── migrations/             # Numbered SQL migration files (.up.sql / .down.sql)
│   │   ├── 000001_init.up.sql
│   │   └── 000001_init.down.sql
│   ├── internal/
│   │   ├── config/             # Environment variable configuration loader
│   │   │   ├── config.go
│   │   │   └── config_test.go
│   │   └── database/           # pgx connection pool & migration runner
│   │       ├── database.go
│   │       └── migrator.go
│   ├── go.mod
│   ├── go.sum
│   ├── .env                    # Real credentials (excluded from git)
│   └── .env.example            # Sample configuration template
│
└── frontend/                   # Managed by the frontend developer
```

---

## 3. Prerequisites

Before running the backend, make sure you have installed:

1. **Go** (`v1.21` or higher, Go 1.27+ recommended)
2. **Docker Desktop** (with Docker Compose support)
3. **PowerShell** (Windows default terminal)

---

## 4. Setup & Running Guide (Windows PowerShell)

### Step 1: Start PostgreSQL with Docker Compose

Open Windows PowerShell at the root of the project (`ctstock/`) and start the PostgreSQL container:

```powershell
# Start PostgreSQL in the background
docker compose up -d
```

Verify that the container is running and healthy:

```powershell
docker compose ps
```

You should see:
```text
NAME                IMAGE                COMMAND                  SERVICE    STATUS
ctstock-postgres    postgres:16-alpine   "docker-entrypoint.s…"   postgres   Up (healthy)
```

> **Data Persistence:** Database data is stored in the Docker volume `ctstock_postgres_data`. When you stop or restart the container, all database records remain intact.

---

### Step 2: Configure Environment Variables

The backend loads configuration from `backend/.env`.

1. Copy `.env.example` to `.env` (if not already created):
   ```powershell
   Copy-Item backend\.env.example backend\.env
   ```

2. Default values inside `backend/.env`:
   ```env
   PORT=8080
   ENV=development
   DB_HOST=localhost
   DB_PORT=5432
   DB_USER=postgres
   DB_PASSWORD=postgrespassword
   DB_NAME=ctstock
   DB_SSLMODE=disable
   DATABASE_URL=postgres://postgres:postgrespassword@localhost:5432/ctstock?sslmode=disable
   ```

*(Optional)* You can also override environment variables directly in PowerShell for the current session:
```powershell
$env:PORT="8080"
$env:DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/ctstock?sslmode=disable"
```

---

### Step 3: Run the Go Backend

Navigate to the `backend/` directory and run:

```powershell
cd backend
go run ./cmd/api
```

You will see:
```text
Starting CTStock Backend on port :8080 (env: development)
Successfully connected to PostgreSQL database
Applied migration: 000001_init.up.sql
Server listening on http://localhost:8080
```

To compile a binary executable:
```powershell
cd backend
go build -o bin/api.exe ./cmd/api
.\bin\api.exe
```

---

### Step 4: Health Check Endpoints

Test the API server using PowerShell or your browser:

#### 1. Server Health Check: `GET /health`
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/health" | ConvertTo-Json
```
**Response (HTTP 200 OK):**
```json
{
  "status": "ok",
  "timestamp": "2026-11-21T18:09:45.9568958+07:00",
  "service": "ctstock-backend"
}
```

#### 2. Database Connectivity Check: `GET /health/db`
```powershell
Invoke-RestMethod -Uri "http://localhost:8080/health/db" | ConvertTo-Json
```
**Response when connected (HTTP 200 OK):**
```json
{
  "status": "ok",
  "database": "connected",
  "timestamp": "2026-11-21T18:15:20.1234567+07:00"
}
```

**Response when disconnected (HTTP 503 Service Unavailable):**
```json
{
  "status": "unhealthy",
  "database": "disconnected",
  "timestamp": "2026-11-21T18:15:20.1234567+07:00",
  "error": "connection refused"
}
```

---

### Step 5: Database Migrations

Migrations are stored in `backend/migrations/` as numbered `.up.sql` and `.down.sql` files:

- `000001_init.up.sql`: Establishes the initial migration tracking baseline.
- `000001_init.down.sql`: Reverts the initial baseline.

**How Migrations Work:**
- When the backend starts, `database.NewMigrationRunner()` scans the `migrations/` folder.
- It creates a `schema_migrations` tracking table in PostgreSQL if it doesn't already exist.
- Each migration is executed inside an atomic transaction (`BEGIN ... COMMIT`).
- Migrations that have already been applied are automatically skipped.

---

### Step 6: Stop Docker Container

When you finish development, stop the PostgreSQL container:

```powershell
# Stop container without removing data
docker compose stop

# Or stop and remove the container (persistent volume remains safe)
docker compose down
```

To delete the database volume and start fresh (caution: deletes all database records):
```powershell
docker compose down -v
```

