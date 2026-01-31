# Council Arena

Self-hosted web application for orchestrating LLM council discussions using GitHub Copilot SDK. Multiple AI models debate, vote, and synthesize answers with comprehensive analytics and ELO ranking system.

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go (Golang) with Fiber or Gin |
| Frontend | Vue.js 3 + Vite + Pinia + Tailwind CSS |
| Database | SQLite (single file, embedded) |
| Auth | GitHub OAuth (required for Copilot SDK) |
| AI Integration | GitHub Copilot SDK (Go) |
| Deployment | Single Docker image |

## Core Concept

Users submit questions to a "council" of AI models. Models respond independently, then vote on each other's responses (blind - they don't know which model wrote what). A chairperson model synthesizes the final answer. All interactions tracked for ELO rankings and analytics.

## Council Flow

```
1. User submits question + selects models + chooses mode
2. Stage 1: All models respond independently (parallel)
3. Stage 2: Models vote on anonymized responses (blind voting)
4. Stage 3: Chairperson synthesizes final answer
5. User provides feedback (thumbs up/down on responses)
6. ELO ratings updated based on votes
```

## Features

### Council Modes

**Standard Mode**
- Single round of responses
- Blind peer voting
- Chairperson synthesis

**Debate Mode**
- Configurable number of rounds (default: 3)
- Models see previous responses and can rebut
- Each round builds on previous arguments
- Final voting after all rounds complete

**Tournament Mode**
- Bracket-style elimination
- Models paired randomly
- Winners advance to next round
- Single champion per session

### Voting & Ranking

**Blind Voting**
- Responses anonymized as "Response A", "Response B", etc.
- Models rank all responses from best to worst
- Models cannot identify other models' responses

**ELO Rating System**
- Chess-style rating (start at 1500)
- Win/loss/draw affects rating
- Separate ELO per category (coding, creative, reasoning, math, general)
- Rating changes based on opponent strength

**User Feedback**
- Thumbs up/down on individual responses
- Affects model ranking with configurable weight
- Tracked separately from model votes

### Special Mechanics

**Devil's Advocate**
- One model randomly assigned to argue against consensus
- Must present counterarguments even if agreeing
- Identified in final report

**Mystery Judge**
- Random model selected as judge only (doesn't participate)
- Provides final ranking without bias
- Judge vote has higher weight

**Minority Report**
- When one model strongly disagrees with consensus
- Dissenting opinion highlighted separately
- Tracked for accuracy over time

**Coalition Detection**
- Detect when models give suspiciously similar responses
- Flag potential "groupthink"
- Diversify council selection recommendation

**Appeal System**
- User can dispute result
- New council convened with different models
- Both verdicts recorded

### Analytics

**Model Statistics**
- Total wins/losses/draws
- Win rate by category
- Average response time
- Average confidence score
- Personality tags (auto-detected: verbose, concise, creative, factual)

**Head-to-Head**
- Direct comparison between any two models
- Historical win rate
- Category-specific performance

**Topic Affinity**
- Which models excel in which categories
- Auto-categorization of questions
- Performance trends

**User Bias Detection**
- Track which models user consistently prefers
- Alert user to potential bias
- Suggest trying other models

**Time Analysis**
- Response quality vs time of day
- API latency tracking
- Identify optimal query times

**Cost Tracking**
- Premium requests used per model
- Cost per session
- Monthly usage summary

### Configuration

**Council Settings**
- Number of models (2-8)
- Model selection (manual or random)
- Chairperson selection (manual, highest ELO, or random)
- Debate rounds (1-10, default 3)
- Response timeout
- Enable/disable specific mechanics

**User Preferences**
- Default council composition
- Preferred categories
- UI density (compact/comfortable)
- Auto-save sessions

## UI Requirements

### Theme
- Primary: Purple (#8B5CF6, #7C3AED, #6D28D9)
- Background: Pure black (#000000) - AMOLED optimized
- Surface: Dark gray (#0A0A0A, #141414)
- Text: White (#FFFFFF) and gray (#A1A1AA)
- Accent: Purple variations for interactive elements
- Error: Red (#EF4444)
- Success: Green (#22C55E)

### Design Principles
- Clean, minimal interface
- No gradients
- No unnecessary animations
- No decorative elements
- High contrast for readability
- Single-column layouts where possible
- All options easily accessible (no deep menus)
- Mobile responsive

### Key Views

**Arena (Main)**
- Question input at top
- Model selector (chips/tags)
- Mode selector (tabs or dropdown)
- Settings panel (collapsible)
- Response cards (during council)
- Voting interface
- Final synthesis

**Rankings**
- Leaderboard table with ELO
- Category tabs
- Sortable columns
- Sparkline charts for trends

**History**
- Session list with filters
- Search functionality
- Session detail view
- Export option

**Analytics**
- Dashboard with key metrics
- Charts for trends
- Head-to-head comparison tool
- Model deep-dive view

**Settings**
- Account (GitHub connection)
- Default preferences
- API usage stats
- Data export/import

## Database Schema

Core tables needed:
- `models` - Available AI models with ELO ratings
- `sessions` - Council sessions
- `responses` - Individual model responses
- `votes` - Model and user votes
- `elo_history` - Rating changes over time
- `matchups` - Head-to-head statistics
- `user_preferences` - User settings
- `categories` - Topic categories

Use SQLite with foreign keys enabled. Single database file for easy backup/migration.

## API Endpoints

### Auth
- `GET /auth/github` - Initiate GitHub OAuth
- `GET /auth/callback` - OAuth callback
- `GET /auth/logout` - Clear session
- `GET /auth/me` - Current user info

### Council
- `POST /api/council/start` - Start new council session
- `GET /api/council/:id` - Get session status/results
- `POST /api/council/:id/vote` - Submit user vote
- `POST /api/council/:id/appeal` - Request appeal

### Models
- `GET /api/models` - List available models
- `GET /api/models/:id` - Model details with stats
- `GET /api/models/:id/history` - ELO history

### Rankings
- `GET /api/rankings` - Global leaderboard
- `GET /api/rankings/:category` - Category leaderboard
- `GET /api/matchups/:modelA/:modelB` - Head-to-head

### Analytics
- `GET /api/analytics/overview` - Dashboard data
- `GET /api/analytics/user-bias` - User preference analysis
- `GET /api/analytics/costs` - Usage costs

### Settings
- `GET /api/settings` - User preferences
- `PUT /api/settings` - Update preferences

## Copilot SDK Integration

The Go backend must:
1. Start Copilot CLI in server mode
2. Communicate via JSON-RPC
3. Manage multiple concurrent sessions (one per model)
4. Handle streaming responses
5. Track token usage for cost calculation

Key SDK operations:
- List available models
- Create session with specific model
- Send prompt and receive response
- Handle tool calls if needed
- Close sessions properly

## Docker Deployment

Single image containing:
- Compiled Go binary
- Built Vue.js static files
- SQLite database (volume mount for persistence)
- Copilot CLI binary

Environment variables:
- `GITHUB_CLIENT_ID` - OAuth app client ID
- `GITHUB_CLIENT_SECRET` - OAuth app secret
- `SESSION_SECRET` - For cookie signing
- `DATABASE_PATH` - SQLite file location (default: /data/council.db)

Ports:
- 8080 (HTTP)

Volume:
- `/data` - Database and any persistent storage

## Development Setup

Requirements:
- Go 1.21+
- Node.js 20+
- GitHub OAuth app configured
- GitHub Copilot subscription
- Copilot CLI installed

## Internationalization (i18n)

### Requirements
- All user-facing text must be translatable
- No hardcoded strings in Vue components or Go templates
- Language selection in UI (persisted in user preferences)
- Browser language auto-detection on first visit

### Base Languages
- English (en) - default
- Polish (pl)

### Implementation

**Frontend (Vue.js)**
- Use vue-i18n
- JSON translation files per language
- Lazy-load language files

**Backend (Go)**
- Return translation keys for error messages
- API responses with translatable content should include keys

### Translation File Structure

```
frontend/src/locales/
├── en.json
├── pl.json
└── _template.json  # Empty template for new languages
```

### Template Format

```json
{
  "_meta": {
    "language": "Language Name",
    "code": "xx",
    "author": "",
    "version": "1.0.0"
  },
  "common": {
    "submit": "",
    "cancel": "",
    "save": "",
    "delete": "",
    "loading": "",
    "error": "",
    "success": ""
  },
  "nav": {
    "arena": "",
    "rankings": "",
    "history": "",
    "analytics": "",
    "settings": ""
  },
  "arena": {
    "title": "",
    "question_placeholder": "",
    "select_models": "",
    "start_council": "",
    "stage_1": "",
    "stage_2": "",
    "stage_3": ""
  },
  "models": {
    "elo_rating": "",
    "wins": "",
    "losses": "",
    "win_rate": ""
  },
  "voting": {
    "vote_for_best": "",
    "submit_vote": "",
    "your_feedback": ""
  },
  "errors": {
    "session_failed": "",
    "model_unavailable": "",
    "timeout": ""
  }
}
```

### Adding New Language

1. Copy `_template.json` to `{code}.json`
2. Fill in `_meta` section
3. Translate all strings
4. Add language to selector in settings
5. No code changes required

## Notes for Implementation

- Use WebSockets for real-time council progress updates
- Implement request queuing to handle Copilot rate limits
- Cache model list (refresh periodically)
- Implement graceful shutdown for ongoing sessions
- Add request timeout handling
- Log all API interactions for debugging
- Implement database migrations for schema updates