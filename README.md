# ImageForge

[中文](README.zh-CN.md)

AI image generation platform with a task queue architecture. Users submit prompts with optional reference images, and remote Runners claim and execute the generation tasks.

## Tech Stack

| Layer    | Stack                                                         |
| -------- | ------------------------------------------------------------- |
| Backend  | Go 1.24, Echo v4, GORM + SQLite, JWT auth, Cobra CLI         |
| Frontend | React 19, Vite 6, Zustand, React Router 7, i18n (CN/EN)      |
| Infra    | Docker multi-stage build, docker-compose                      |

## Quick Start

### Docker (recommended)

```bash
cp .env.example .env
# Edit .env and set IMAGEFORGE_JWT_SECRET (generate with: openssl rand -hex 32)

docker compose up --build
```

The app will be available at `http://localhost:8020`.

### Development

**Backend** (with hot reload via [Air](https://github.com/air-verse/air)):

```bash
cd backend
go run ./cmd/server
```

**Frontend**:

```bash
cd frontend
npm install
npm run dev
```

The frontend dev server proxies API requests to the backend. For production, `npm run build` outputs static files that are embedded into the Go binary.

## Project Structure

```
ImageForge
├── backend/           # Go API server + CLI
│   ├── cmd/
│   │   ├── server/    # HTTP server entrypoint
│   │   └── cli/       # CLI tool
│   └── internal/      # Handlers, models, config, middleware
├── frontend/          # React SPA
│   └── src/
├── data/              # Runtime data (SQLite DB, images)
├── docs/              # Design docs
└── docker-compose.yml
```

## Environment Variables

| Variable              | Required | Description                          |
| --------------------- | -------- | ------------------------------------ |
| `IMAGEFORGE_JWT_SECRET` | Yes    | JWT signing key                      |
| `DATA_DIR`            | No       | Host data directory (default `./data`) |
| `PORT`                | No       | Host port mapping (default `8020`)   |

See [.env.example](.env.example) for the full reference.

## API Overview

- **Auth** -- JWT-based login with rate limiting
- **Tasks** -- Create, list, cancel image generation tasks; upload reference images
- **Runners** -- Register, heartbeat, claim tasks, submit results

## License

[MIT](LICENSE)
