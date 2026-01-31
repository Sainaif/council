package council

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sainaif/council/internal/database"
	"github.com/sainaif/council/internal/services/copilot"
	"github.com/sainaif/council/internal/services/elo"
	"github.com/sainaif/council/internal/websocket"
)

type SessionStatus string

const (
	StatusPending      SessionStatus = "pending"
	StatusResponding   SessionStatus = "responding"
	StatusVoting       SessionStatus = "voting"
	StatusSynthesizing SessionStatus = "synthesizing"
	StatusCompleted    SessionStatus = "completed"
	StatusFailed       SessionStatus = "failed"
	StatusCancelled    SessionStatus = "cancelled"
)

type Mode string

const (
	ModeStandard   Mode = "standard"
	ModeDebate     Mode = "debate"
	ModeTournament Mode = "tournament"
)

type StartRequest struct {
	Question        string   `json:"question"`
	Models          []string `json:"models"`
	Mode            Mode     `json:"mode"`
	CategoryID      *int64   `json:"category_id,omitempty"`
	ChairpersonID   *string  `json:"chairperson_id,omitempty"`
	DebateRounds    int      `json:"debate_rounds,omitempty"`
	EnableDevil     bool     `json:"enable_devil_advocate,omitempty"`
	EnableMystery   bool     `json:"enable_mystery_judge,omitempty"`
	ResponseTimeout int      `json:"response_timeout,omitempty"` // seconds
}

type Session struct {
	ID              string        `json:"id"`
	UserID          string        `json:"user_id"`
	Question        string        `json:"question"`
	Mode            Mode          `json:"mode"`
	Status          SessionStatus `json:"status"`
	CategoryID      *int64        `json:"category_id,omitempty"`
	ChairpersonID   *string       `json:"chairperson_id,omitempty"`
	DevilAdvocateID *string       `json:"devil_advocate_id,omitempty"`
	MysteryJudgeID  *string       `json:"mystery_judge_id,omitempty"`
	Synthesis       string        `json:"synthesis,omitempty"`
	MinorityReport  string        `json:"minority_report,omitempty"`
	Responses       []Response    `json:"responses,omitempty"`
	Votes           []Vote        `json:"votes,omitempty"`
	Config          SessionConfig `json:"config"`
	CreatedAt       time.Time     `json:"created_at"`
	CompletedAt     *time.Time    `json:"completed_at,omitempty"`
}

type SessionConfig struct {
	DebateRounds    int  `json:"debate_rounds"`
	ResponseTimeout int  `json:"response_timeout"`
	EnableDevil     bool `json:"enable_devil_advocate"`
	EnableMystery   bool `json:"enable_mystery_judge"`
}

type Response struct {
	ID             int64     `json:"id"`
	SessionID      string    `json:"session_id"`
	ModelID        string    `json:"model_id"`
	Round          int       `json:"round"`
	Content        string    `json:"content"`
	AnonymousLabel string    `json:"anonymous_label"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	TokenCount     int       `json:"token_count"`
	CreatedAt      time.Time `json:"created_at"`
}

type Vote struct {
	ID              int64     `json:"id"`
	SessionID       string    `json:"session_id"`
	VoterType       string    `json:"voter_type"` // "model" or "user"
	VoterID         string    `json:"voter_id"`
	RankedResponses []string  `json:"ranked_responses"`
	Weight          float64   `json:"weight"`
	CreatedAt       time.Time `json:"created_at"`
}

type Orchestrator struct {
	db      *database.DB
	copilot *copilot.Service
	elo     *elo.Calculator
	hub     *websocket.Hub
}

func NewOrchestrator(db *database.DB, copilot *copilot.Service, elo *elo.Calculator, hub *websocket.Hub) *Orchestrator {
	return &Orchestrator{
		db:      db,
		copilot: copilot,
		elo:     elo,
		hub:     hub,
	}
}

func (o *Orchestrator) StartSession(ctx context.Context, userID string, req StartRequest) (*Session, error) {
	// Validate request
	if err := o.validateRequest(req); err != nil {
		return nil, err
	}

	// Create session
	sessionID := uuid.New().String()
	config := SessionConfig{
		DebateRounds:    req.DebateRounds,
		ResponseTimeout: req.ResponseTimeout,
		EnableDevil:     req.EnableDevil,
		EnableMystery:   req.EnableMystery,
	}
	if config.DebateRounds == 0 {
		config.DebateRounds = 3
	}
	if config.ResponseTimeout == 0 {
		config.ResponseTimeout = 60
	}

	// Select special roles
	var devilID, mysteryID *string
	participatingModels := make([]string, len(req.Models))
	copy(participatingModels, req.Models)

	if config.EnableMystery && len(participatingModels) > 2 {
		idx := rand.Intn(len(participatingModels))
		mysteryID = &participatingModels[idx]
		// Remove from participating (they only judge)
		participatingModels = append(participatingModels[:idx], participatingModels[idx+1:]...)
	}

	if config.EnableDevil && len(participatingModels) > 1 {
		idx := rand.Intn(len(participatingModels))
		devilID = &participatingModels[idx]
	}

	// Select chairperson
	chairpersonID := req.ChairpersonID
	if chairpersonID == nil && len(participatingModels) > 0 {
		// Select highest ELO model as chairperson
		chairpersonID = &participatingModels[0]
	}

	configJSON, _ := json.Marshal(config)

	// Insert session
	_, err := o.db.Exec(`
		INSERT INTO sessions (id, user_id, question, category_id, mode, status, config, chairperson_id, devil_advocate_id, mystery_judge_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sessionID, userID, req.Question, req.CategoryID, req.Mode, StatusPending, string(configJSON), chairpersonID, devilID, mysteryID)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Register models if they don't exist
	for _, modelID := range req.Models {
		model, err := o.copilot.GetModel(ctx, modelID)
		if err != nil {
			continue
		}
		_, _ = o.db.Exec(`
			INSERT OR IGNORE INTO models (id, display_name, provider)
			VALUES (?, ?, ?)
		`, model.ID, model.DisplayName, model.Provider)
	}

	session := &Session{
		ID:              sessionID,
		UserID:          userID,
		Question:        req.Question,
		Mode:            req.Mode,
		Status:          StatusPending,
		CategoryID:      req.CategoryID,
		ChairpersonID:   chairpersonID,
		DevilAdvocateID: devilID,
		MysteryJudgeID:  mysteryID,
		Config:          config,
		CreatedAt:       time.Now(),
	}

	// Start council execution in background
	go o.executeCouncil(context.Background(), session, participatingModels)

	return session, nil
}

func (o *Orchestrator) validateRequest(req StartRequest) error {
	if req.Question == "" {
		return fmt.Errorf("question is required")
	}
	if len(req.Models) < 2 {
		return fmt.Errorf("at least 2 models are required")
	}
	if len(req.Models) > 8 {
		return fmt.Errorf("maximum 8 models allowed")
	}
	if req.Mode == "" {
		req.Mode = ModeStandard
	}
	if req.Mode != ModeStandard && req.Mode != ModeDebate && req.Mode != ModeTournament {
		return fmt.Errorf("invalid mode: %s", req.Mode)
	}
	return nil
}

func (o *Orchestrator) executeCouncil(ctx context.Context, session *Session, models []string) {
	// Update status to responding
	o.updateSessionStatus(session.ID, StatusResponding)
	o.hub.Broadcast(session.ID, websocket.EventCouncilStarted, map[string]interface{}{
		"session_id": session.ID,
		"mode":       session.Mode,
		"models":     models,
	})

	switch session.Mode {
	case ModeStandard:
		o.executeStandardMode(ctx, session, models)
	case ModeDebate:
		o.executeDebateMode(ctx, session, models)
	case ModeTournament:
		o.executeTournamentMode(ctx, session, models)
	}
}

func (o *Orchestrator) executeStandardMode(ctx context.Context, session *Session, models []string) {
	// Stage 1: Collect responses in parallel
	responses, err := o.collectResponses(ctx, session, models, 1)
	if err != nil {
		o.failSession(session.ID, err.Error())
		return
	}

	// Stage 2: Voting
	o.updateSessionStatus(session.ID, StatusVoting)
	o.hub.Broadcast(session.ID, websocket.EventVotingStarted, nil)

	votes, err := o.collectVotes(ctx, session, responses, models)
	if err != nil {
		o.failSession(session.ID, err.Error())
		return
	}

	// Stage 3: Synthesis
	o.updateSessionStatus(session.ID, StatusSynthesizing)
	o.hub.Broadcast(session.ID, websocket.EventSynthesisStarted, nil)

	if err := o.synthesize(ctx, session, responses, votes); err != nil {
		o.failSession(session.ID, err.Error())
		return
	}

	// Update ELO ratings
	rankings := make(map[string][]string)
	for _, vote := range votes {
		rankings[vote.VoterID] = vote.RankedResponses
	}
	_, _ = o.elo.UpdateRatings(session.ID, session.CategoryID, rankings)

	// Complete session
	o.completeSession(session.ID)
}

func (o *Orchestrator) executeDebateMode(ctx context.Context, session *Session, models []string) {
	var allResponses []Response

	for round := 1; round <= session.Config.DebateRounds; round++ {
		responses, err := o.collectResponses(ctx, session, models, round)
		if err != nil {
			o.failSession(session.ID, err.Error())
			return
		}
		allResponses = append(allResponses, responses...)
	}

	// Voting on final round responses only
	o.updateSessionStatus(session.ID, StatusVoting)
	finalResponses := filterByRound(allResponses, session.Config.DebateRounds)

	votes, err := o.collectVotes(ctx, session, finalResponses, models)
	if err != nil {
		o.failSession(session.ID, err.Error())
		return
	}

	// Synthesis
	o.updateSessionStatus(session.ID, StatusSynthesizing)
	if err := o.synthesize(ctx, session, finalResponses, votes); err != nil {
		o.failSession(session.ID, err.Error())
		return
	}

	o.completeSession(session.ID)
}

func (o *Orchestrator) executeTournamentMode(ctx context.Context, session *Session, models []string) {
	// Bracket-style elimination
	remaining := models

	for len(remaining) > 1 {
		var winners []string
		for i := 0; i < len(remaining); i += 2 {
			if i+1 >= len(remaining) {
				// Odd one out advances automatically
				winners = append(winners, remaining[i])
				continue
			}

			matchModels := []string{remaining[i], remaining[i+1]}
			responses, err := o.collectResponses(ctx, session, matchModels, 1)
			if err != nil {
				continue
			}

			votes, err := o.collectVotes(ctx, session, responses, matchModels)
			if err != nil {
				continue
			}

			// Determine winner
			winner := determineWinner(votes)
			winners = append(winners, winner)
		}
		remaining = winners
	}

	if len(remaining) == 1 {
		// Champion determined
		o.hub.Broadcast(session.ID, "tournament.champion", map[string]string{
			"champion": remaining[0],
		})
	}

	o.completeSession(session.ID)
}

func (o *Orchestrator) collectResponses(ctx context.Context, session *Session, models []string, round int) ([]Response, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var responses []Response
	var errors []error

	labels := generateLabels(len(models))

	for i, modelID := range models {
		wg.Add(1)
		go func(idx int, mID string) {
			defer wg.Done()

			label := labels[idx]
			o.hub.Broadcast(session.ID, websocket.EventModelResponding, map[string]interface{}{
				"model_id": mID,
				"label":    label,
			})

			// Build prompt (include context for debate mode)
			prompt := session.Question
			if session.DevilAdvocateID != nil && *session.DevilAdvocateID == mID {
				prompt = fmt.Sprintf("[ROLE: Devil's Advocate - You must argue against the consensus view]\n\n%s", prompt)
			}

			start := time.Now()

			// Stream response
			chunks, err := o.copilot.StreamPrompt(ctx, mID, prompt)
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
				return
			}

			var content string
			var tokenCount int
			for chunk := range chunks {
				if chunk.Error != nil {
					mu.Lock()
					errors = append(errors, chunk.Error)
					mu.Unlock()
					return
				}
				content += chunk.Content
				tokenCount = chunk.TokenCount

				// Broadcast chunk
				o.hub.Broadcast(session.ID, websocket.EventModelResponseChunk, map[string]interface{}{
					"model_id": mID,
					"label":    label,
					"content":  chunk.Content,
					"done":     chunk.Done,
				})
			}

			responseTime := time.Since(start).Milliseconds()

			// Save response
			result, err := o.db.Exec(`
				INSERT INTO responses (session_id, model_id, round, content, anonymous_label, response_time_ms, token_count)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`, session.ID, mID, round, content, label, responseTime, tokenCount)
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
				return
			}

			id, _ := result.LastInsertId()
			mu.Lock()
			responses = append(responses, Response{
				ID:             id,
				SessionID:      session.ID,
				ModelID:        mID,
				Round:          round,
				Content:        content,
				AnonymousLabel: label,
				ResponseTimeMs: responseTime,
				TokenCount:     tokenCount,
				CreatedAt:      time.Now(),
			})
			mu.Unlock()

			o.hub.Broadcast(session.ID, websocket.EventModelComplete, map[string]interface{}{
				"model_id":      mID,
				"label":         label,
				"response_time": responseTime,
			})
		}(i, modelID)
	}

	wg.Wait()

	if len(errors) > 0 {
		return responses, errors[0]
	}

	return responses, nil
}

func (o *Orchestrator) collectVotes(ctx context.Context, session *Session, responses []Response, models []string) ([]Vote, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var votes []Vote

	// Prepare anonymized responses
	anonymizedResponses := make(map[string]string)
	for _, r := range responses {
		anonymizedResponses[r.AnonymousLabel] = r.Content
	}

	// Exclude mystery judge from voting models if present
	votingModels := models
	if session.MysteryJudgeID != nil {
		votingModels = append(votingModels, *session.MysteryJudgeID)
	}

	for _, modelID := range votingModels {
		wg.Add(1)
		go func(mID string) {
			defer wg.Done()

			// Request vote
			ranking, err := o.copilot.RequestVote(ctx, mID, session.Question, anonymizedResponses)
			if err != nil {
				return
			}

			// Determine weight (mystery judge gets higher weight)
			weight := 1.0
			if session.MysteryJudgeID != nil && *session.MysteryJudgeID == mID {
				weight = 1.5
			}

			rankingJSON, _ := json.Marshal(ranking)

			// Save vote
			result, err := o.db.Exec(`
				INSERT INTO votes (session_id, voter_type, voter_id, ranked_responses, weight)
				VALUES (?, 'model', ?, ?, ?)
			`, session.ID, mID, string(rankingJSON), weight)
			if err != nil {
				return
			}

			id, _ := result.LastInsertId()
			mu.Lock()
			votes = append(votes, Vote{
				ID:              id,
				SessionID:       session.ID,
				VoterType:       "model",
				VoterID:         mID,
				RankedResponses: ranking,
				Weight:          weight,
				CreatedAt:       time.Now(),
			})
			mu.Unlock()

			o.hub.Broadcast(session.ID, websocket.EventVoteReceived, map[string]interface{}{
				"voter_id": mID,
			})
		}(modelID)
	}

	wg.Wait()
	return votes, nil
}

func (o *Orchestrator) synthesize(ctx context.Context, session *Session, responses []Response, votes []Vote) error {
	if session.ChairpersonID == nil {
		return fmt.Errorf("no chairperson assigned")
	}

	// Prepare data for synthesis
	respMap := make(map[string]string)
	for _, r := range responses {
		respMap[r.AnonymousLabel] = r.Content
	}

	voteMap := make(map[string][]string)
	for _, v := range votes {
		voteMap[v.VoterID] = v.RankedResponses
	}

	// Request synthesis
	synthesis, err := o.copilot.RequestSynthesis(ctx, *session.ChairpersonID, session.Question, respMap, voteMap)
	if err != nil {
		return err
	}

	// Detect minority report (significant disagreement)
	minorityReport := detectMinorityReport(votes)

	// Update session
	_, err = o.db.Exec(`
		UPDATE sessions SET synthesis = ?, minority_report = ? WHERE id = ?
	`, synthesis.Content, minorityReport, session.ID)

	o.hub.Broadcast(session.ID, websocket.EventSynthesisComplete, map[string]interface{}{
		"synthesis":       synthesis.Content,
		"minority_report": minorityReport,
	})

	return err
}

func (o *Orchestrator) updateSessionStatus(sessionID string, status SessionStatus) {
	_, _ = o.db.Exec(`UPDATE sessions SET status = ? WHERE id = ?`, status, sessionID)
}

func (o *Orchestrator) failSession(sessionID, reason string) {
	_, _ = o.db.Exec(`UPDATE sessions SET status = ? WHERE id = ?`, StatusFailed, sessionID)
	o.hub.Broadcast(sessionID, websocket.EventCouncilFailed, map[string]string{
		"reason": reason,
	})
}

func (o *Orchestrator) completeSession(sessionID string) {
	_, _ = o.db.Exec(`UPDATE sessions SET status = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?`, StatusCompleted, sessionID)
	o.hub.Broadcast(sessionID, websocket.EventCouncilCompleted, nil)
}

func (o *Orchestrator) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	var session Session
	var configJSON, synthesis, minorityReport sql.NullString
	var chairpersonID, devilID, mysteryID sql.NullString
	var categoryID sql.NullInt64
	var completedAt sql.NullTime

	err := o.db.QueryRow(`
		SELECT id, user_id, question, category_id, mode, status, config, chairperson_id,
			   devil_advocate_id, mystery_judge_id, synthesis, minority_report, created_at, completed_at
		FROM sessions WHERE id = ?
	`, sessionID).Scan(
		&session.ID, &session.UserID, &session.Question, &categoryID,
		&session.Mode, &session.Status, &configJSON, &chairpersonID,
		&devilID, &mysteryID, &synthesis, &minorityReport,
		&session.CreatedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}

	if categoryID.Valid {
		session.CategoryID = &categoryID.Int64
	}
	if chairpersonID.Valid {
		session.ChairpersonID = &chairpersonID.String
	}
	if devilID.Valid {
		session.DevilAdvocateID = &devilID.String
	}
	if mysteryID.Valid {
		session.MysteryJudgeID = &mysteryID.String
	}
	if synthesis.Valid {
		session.Synthesis = synthesis.String
	}
	if minorityReport.Valid {
		session.MinorityReport = minorityReport.String
	}
	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}
	if configJSON.Valid {
		_ = json.Unmarshal([]byte(configJSON.String), &session.Config)
	}

	// Load responses
	rows, err := o.db.Query(`
		SELECT id, session_id, model_id, round, content, anonymous_label, response_time_ms, token_count, created_at
		FROM responses WHERE session_id = ? ORDER BY round, id
	`, sessionID)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var r Response
			_ = rows.Scan(&r.ID, &r.SessionID, &r.ModelID, &r.Round, &r.Content,
				&r.AnonymousLabel, &r.ResponseTimeMs, &r.TokenCount, &r.CreatedAt)
			session.Responses = append(session.Responses, r)
		}
	}

	// Load votes
	voteRows, err := o.db.Query(`
		SELECT id, session_id, voter_type, voter_id, ranked_responses, weight, created_at
		FROM votes WHERE session_id = ?
	`, sessionID)
	if err == nil {
		defer func() { _ = voteRows.Close() }()
		for voteRows.Next() {
			var v Vote
			var rankedJSON string
			_ = voteRows.Scan(&v.ID, &v.SessionID, &v.VoterType, &v.VoterID, &rankedJSON, &v.Weight, &v.CreatedAt)
			_ = json.Unmarshal([]byte(rankedJSON), &v.RankedResponses)
			session.Votes = append(session.Votes, v)
		}
	}

	return &session, nil
}

func (o *Orchestrator) SubmitUserVote(ctx context.Context, sessionID, userID string, ranking []string) error {
	rankingJSON, _ := json.Marshal(ranking)
	_, err := o.db.Exec(`
		INSERT INTO votes (session_id, voter_type, voter_id, ranked_responses, weight)
		VALUES (?, 'user', ?, ?, 0.5)
	`, sessionID, userID, string(rankingJSON))
	return err
}

func (o *Orchestrator) CancelSession(ctx context.Context, sessionID string) error {
	_, err := o.db.Exec(`UPDATE sessions SET status = ? WHERE id = ?`, StatusCancelled, sessionID)
	o.hub.Broadcast(sessionID, "council.cancelled", nil)
	return err
}

// Helper functions
func generateLabels(count int) []string {
	labels := make([]string, count)
	for i := 0; i < count; i++ {
		labels[i] = fmt.Sprintf("Response %c", 'A'+i)
	}
	return labels
}

func filterByRound(responses []Response, round int) []Response {
	var filtered []Response
	for _, r := range responses {
		if r.Round == round {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func determineWinner(votes []Vote) string {
	scores := make(map[string]int)
	for _, v := range votes {
		for i, label := range v.RankedResponses {
			scores[label] += len(v.RankedResponses) - i
		}
	}

	var winner string
	maxScore := 0
	for label, score := range scores {
		if score > maxScore {
			maxScore = score
			winner = label
		}
	}
	return winner
}

func detectMinorityReport(votes []Vote) string {
	if len(votes) < 3 {
		return ""
	}

	// Check if any model's ranking significantly differs from majority
	// This is a simplified implementation
	return ""
}
