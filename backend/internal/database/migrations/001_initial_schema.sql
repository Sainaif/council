-- +goose Up
-- +goose StatementBegin

-- Enable foreign keys for SQLite
PRAGMA foreign_keys = ON;

-- Categories for topic-specific ELO
CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Seed default categories
INSERT INTO categories (name, description) VALUES
    ('general', 'General knowledge and miscellaneous topics'),
    ('coding', 'Programming, software development, and technical problems'),
    ('creative', 'Creative writing, brainstorming, and artistic content'),
    ('reasoning', 'Logic, analysis, and problem-solving'),
    ('math', 'Mathematics, statistics, and calculations');

-- AI Models registry
CREATE TABLE models (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    provider TEXT NOT NULL,
    is_active BOOLEAN DEFAULT 1,
    personality_tags TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ELO ratings per model per category
CREATE TABLE model_ratings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    model_id TEXT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    rating INTEGER DEFAULT 1500,
    wins INTEGER DEFAULT 0,
    losses INTEGER DEFAULT 0,
    draws INTEGER DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(model_id, category_id)
);

-- Council sessions
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    question TEXT NOT NULL,
    category_id INTEGER REFERENCES categories(id),
    mode TEXT NOT NULL CHECK(mode IN ('standard', 'debate', 'tournament')),
    status TEXT NOT NULL CHECK(status IN ('pending', 'responding', 'voting', 'synthesizing', 'completed', 'failed', 'cancelled')),
    config TEXT,
    chairperson_id TEXT REFERENCES models(id),
    synthesis TEXT,
    devil_advocate_id TEXT REFERENCES models(id),
    mystery_judge_id TEXT REFERENCES models(id),
    minority_report TEXT,
    appeal_session_id TEXT REFERENCES sessions(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

-- Model responses within sessions
CREATE TABLE responses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL REFERENCES models(id),
    round INTEGER DEFAULT 1,
    content TEXT NOT NULL,
    reasoning TEXT,
    response_time_ms INTEGER,
    token_count INTEGER,
    anonymous_label TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Votes (both model and user votes)
CREATE TABLE votes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    voter_type TEXT NOT NULL CHECK(voter_type IN ('model', 'user')),
    voter_id TEXT NOT NULL,
    ranked_responses TEXT NOT NULL,
    weight REAL DEFAULT 1.0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ELO history for tracking changes over time
CREATE TABLE elo_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    model_id TEXT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories(id),
    session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
    old_rating INTEGER NOT NULL,
    new_rating INTEGER NOT NULL,
    change INTEGER NOT NULL,
    reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Head-to-head matchup statistics
CREATE TABLE matchups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    model_a_id TEXT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    model_b_id TEXT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories(id),
    model_a_wins INTEGER DEFAULT 0,
    model_b_wins INTEGER DEFAULT 0,
    draws INTEGER DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(model_a_id, model_b_id, category_id)
);

-- User preferences and settings
CREATE TABLE user_preferences (
    user_id TEXT PRIMARY KEY,
    github_username TEXT NOT NULL,
    github_avatar_url TEXT,
    default_models TEXT,
    preferred_categories TEXT,
    ui_density TEXT DEFAULT 'comfortable' CHECK(ui_density IN ('compact', 'comfortable')),
    language TEXT DEFAULT 'en',
    auto_save_sessions BOOLEAN DEFAULT 1,
    user_feedback_weight REAL DEFAULT 0.5,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_created ON sessions(created_at);
CREATE INDEX idx_responses_session ON responses(session_id);
CREATE INDEX idx_votes_session ON votes(session_id);
CREATE INDEX idx_elo_history_model ON elo_history(model_id);
CREATE INDEX idx_elo_history_created ON elo_history(created_at);
CREATE INDEX idx_model_ratings_model ON model_ratings(model_id);
CREATE INDEX idx_matchups_models ON matchups(model_a_id, model_b_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_matchups_models;
DROP INDEX IF EXISTS idx_model_ratings_model;
DROP INDEX IF EXISTS idx_elo_history_created;
DROP INDEX IF EXISTS idx_elo_history_model;
DROP INDEX IF EXISTS idx_votes_session;
DROP INDEX IF EXISTS idx_responses_session;
DROP INDEX IF EXISTS idx_sessions_created;
DROP INDEX IF EXISTS idx_sessions_status;
DROP INDEX IF EXISTS idx_sessions_user;

DROP TABLE IF EXISTS user_preferences;
DROP TABLE IF EXISTS matchups;
DROP TABLE IF EXISTS elo_history;
DROP TABLE IF EXISTS votes;
DROP TABLE IF EXISTS responses;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS model_ratings;
DROP TABLE IF EXISTS models;
DROP TABLE IF EXISTS categories;

-- +goose StatementEnd
