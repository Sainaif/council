# Council Arena

Self-hosted web application for orchestrating LLM council discussions using GitHub Copilot SDK. Multiple AI models debate, vote, and synthesize answers with comprehensive analytics and ELO ranking system.

## Features

- **Council Modes**: Standard, Debate, and Tournament modes for different discussion styles
- **Blind Voting**: Models vote on anonymized responses without knowing authorship
- **ELO Ranking**: Chess-style rating system tracking model performance by category
- **Special Mechanics**: Devil's Advocate, Mystery Judge, Minority Report, and more
- **Analytics Dashboard**: Comprehensive statistics, head-to-head comparisons, and trends
- **Full i18n**: English and Polish support with easy extensibility

## Quick Start

### Prerequisites

- Docker and Docker Compose
- GitHub account with Copilot subscription

### Using Prebuilt Image (Recommended)

The prebuilt image includes built-in OAuth credentials, so you can run it immediately:

```bash
cd docker
docker compose up -d
```

**Access the application** at `http://localhost:8080`

#### Optional: Use Your Own OAuth App

If you want to use your own GitHub OAuth application instead of the built-in defaults:

1. Create a `.env` file from the example:
   ```bash
   cp .env.example .env
   ```

2. Configure your OAuth credentials in `.env`:
   ```env
   GITHUB_CLIENT_ID=your_client_id
   GITHUB_CLIENT_SECRET=your_client_secret
   ```

3. Uncomment the OAuth environment variables in `docker-compose.yml`

### Building from Source

If you want to build the image yourself (e.g., to embed your own OAuth credentials at build time):

1. Edit `docker-compose.yml` and uncomment the `build` section
2. Run:
   ```bash
   cd docker
   docker compose up -d --build
   ```

## Configuration

All configuration is done via environment variables. See [.env.example](.env.example) for all available options.

| Variable | Description | Default |
|----------|-------------|---------|
| `GITHUB_CLIENT_ID` | GitHub OAuth App Client ID | Built-in |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth App Secret | Built-in |
| `SESSION_SECRET` | Secret for session signing | Auto-generated |
| `DATABASE_PATH` | SQLite database location | `/data/council.db` |
| `PORT` | HTTP server port | `8080` |
| `ENV` | Environment mode | `production` |

## Development

### Requirements

- Go 1.25+
- Node.js 25+
- GitHub OAuth app configured
- GitHub Copilot subscription

### Setup

```bash
# Clone the repository
git clone https://github.com/Sainaif/council.git
cd council

# Run setup script
./scripts/setup.sh
```

### Manual Development Setup

**Frontend:**
```bash
cd frontend
npm install
npm run dev
```

**Backend:**
```bash
cd backend
go mod download
go run ./cmd/council
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Council Arena                         │
├─────────────────────────────────────────────────────────────┤
│  Frontend (Vue 3 + Vite + Pinia + Tailwind CSS)             │
├─────────────────────────────────────────────────────────────┤
│  Backend (Go + Fiber)                                        │
│  ├── GitHub OAuth Authentication                             │
│  ├── GitHub Copilot SDK Integration                          │
│  ├── WebSocket for Real-time Updates                         │
│  └── ELO Rating Calculator                                   │
├─────────────────────────────────────────────────────────────┤
│  SQLite Database (Single File)                               │
└─────────────────────────────────────────────────────────────┘
```

## Council Flow

```
1. User submits question + selects models + chooses mode
2. Stage 1: All models respond independently (parallel)
3. Stage 2: Models vote on anonymized responses (blind voting)
4. Stage 3: Chairperson synthesizes final answer
5. User provides feedback (thumbs up/down on responses)
6. ELO ratings updated based on votes
```

## API Endpoints

### Authentication
- `GET /auth/github` - Initiate GitHub OAuth
- `GET /auth/callback` - OAuth callback
- `GET /auth/logout` - Clear session
- `GET /auth/me` - Current user info

### Council
- `POST /api/council/start` - Start new council session
- `GET /api/council/:id` - Get session status/results
- `POST /api/council/:id/vote` - Submit user vote
- `POST /api/council/:id/appeal` - Request appeal

### Models & Rankings
- `GET /api/models` - List available models
- `GET /api/rankings` - Global leaderboard
- `GET /api/matchups/:modelA/:modelB` - Head-to-head comparison

### Analytics
- `GET /api/analytics/overview` - Dashboard data
- `GET /api/analytics/user-bias` - User preference analysis
- `GET /api/analytics/costs` - Usage costs

## License

MIT
