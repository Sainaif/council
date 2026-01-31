package elo

import (
	"database/sql"
	"math"

	"github.com/sainaif/council/internal/database"
)

const (
	InitialRating = 1500
	KFactorNew    = 25 // Players with < 30 games
	KFactorNormal = 15 // Regular players
	KFactorPro    = 10 // Top performers (rating > 2000)
)

type Calculator struct {
	db *database.DB
}

type RatingChange struct {
	ModelID    string `json:"model_id"`
	OldRating  int    `json:"old_rating"`
	NewRating  int    `json:"new_rating"`
	Change     int    `json:"change"`
	CategoryID *int64 `json:"category_id,omitempty"`
}

func NewCalculator(db *database.DB) *Calculator {
	return &Calculator{db: db}
}

// ExpectedScore calculates the expected score using the logistic function
func ExpectedScore(ratingA, ratingB int) float64 {
	return 1.0 / (1.0 + math.Pow(10, float64(ratingB-ratingA)/400))
}

// GetKFactor determines the K-factor based on games played and rating
func GetKFactor(gamesPlayed, rating int) int {
	if gamesPlayed < 30 {
		return KFactorNew
	}
	if rating > 2000 {
		return KFactorPro
	}
	return KFactorNormal
}

// UpdateRatings updates ELO ratings based on voting results
// rankings maps voter to their ordered list of model IDs (best first)
func (c *Calculator) UpdateRatings(sessionID string, categoryID *int64, rankings map[string][]string) ([]RatingChange, error) {
	var changes []RatingChange

	// Extract all models from rankings
	models := make(map[string]bool)
	for _, ranking := range rankings {
		for _, modelID := range ranking {
			models[modelID] = true
		}
	}

	// Get current ratings
	currentRatings := make(map[string]int)
	gamesPlayed := make(map[string]int)

	for modelID := range models {
		rating, games, err := c.getModelRating(modelID, categoryID)
		if err != nil {
			return nil, err
		}
		currentRatings[modelID] = rating
		gamesPlayed[modelID] = games
	}

	// Calculate pairwise results
	pairResults := make(map[string]map[string]float64) // modelA -> modelB -> score (1=win, 0.5=draw, 0=loss)
	for modelID := range models {
		pairResults[modelID] = make(map[string]float64)
	}

	// Process each ranking to create pairwise comparisons
	for _, ranking := range rankings {
		for i := 0; i < len(ranking); i++ {
			for j := i + 1; j < len(ranking); j++ {
				winner := ranking[i]
				loser := ranking[j]

				// Winner gets a point against loser
				pairResults[winner][loser] += 1.0
				pairResults[loser][winner] += 0.0
			}
		}
	}

	// Calculate new ratings
	newRatings := make(map[string]float64)
	for modelID := range models {
		newRatings[modelID] = float64(currentRatings[modelID])
	}

	// Apply ELO adjustments for each pairwise matchup
	numVoters := float64(len(rankings))
	for modelA := range models {
		for modelB, score := range pairResults[modelA] {
			if modelA >= modelB {
				continue // Process each pair only once
			}

			scoreA := score / numVoters
			scoreB := pairResults[modelB][modelA] / numVoters

			ratingA := currentRatings[modelA]
			ratingB := currentRatings[modelB]

			expectedA := ExpectedScore(ratingA, ratingB)
			expectedB := 1 - expectedA

			kA := float64(GetKFactor(gamesPlayed[modelA], ratingA))
			kB := float64(GetKFactor(gamesPlayed[modelB], ratingB))

			newRatings[modelA] += kA * (scoreA - expectedA)
			newRatings[modelB] += kB * (scoreB - expectedB)
		}
	}

	// Update database and collect changes
	err := c.db.WithTx(func(tx *sql.Tx) error {
		for modelID := range models {
			oldRating := currentRatings[modelID]
			newRating := int(math.Round(newRatings[modelID]))
			change := newRating - oldRating

			// Determine win/loss/draw counts
			wins, losses, draws := 0, 0, 0
			for otherModel, score := range pairResults[modelID] {
				if otherModel == modelID {
					continue
				}
				avgScore := score / numVoters
				if avgScore > 0.6 {
					wins++
				} else if avgScore < 0.4 {
					losses++
				} else {
					draws++
				}
			}

			// Update model_ratings
			if err := c.updateModelRating(tx, modelID, categoryID, newRating, wins, losses, draws); err != nil {
				return err
			}

			// Record history
			if err := c.recordHistory(tx, modelID, categoryID, sessionID, oldRating, newRating, change); err != nil {
				return err
			}

			changes = append(changes, RatingChange{
				ModelID:    modelID,
				OldRating:  oldRating,
				NewRating:  newRating,
				Change:     change,
				CategoryID: categoryID,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return changes, nil
}

func (c *Calculator) getModelRating(modelID string, categoryID *int64) (int, int, error) {
	var rating, wins, losses, draws int

	var query string
	var args []interface{}

	if categoryID != nil {
		query = `SELECT COALESCE(rating, ?), COALESCE(wins, 0), COALESCE(losses, 0), COALESCE(draws, 0)
				 FROM model_ratings WHERE model_id = ? AND category_id = ?`
		args = []interface{}{InitialRating, modelID, *categoryID}
	} else {
		query = `SELECT COALESCE(rating, ?), COALESCE(wins, 0), COALESCE(losses, 0), COALESCE(draws, 0)
				 FROM model_ratings WHERE model_id = ? AND category_id IS NULL`
		args = []interface{}{InitialRating, modelID}
	}

	err := c.db.QueryRow(query, args...).Scan(&rating, &wins, &losses, &draws)
	if err == sql.ErrNoRows {
		return InitialRating, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}

	return rating, wins + losses + draws, nil
}

func (c *Calculator) updateModelRating(tx *sql.Tx, modelID string, categoryID *int64, rating, wins, losses, draws int) error {
	if categoryID != nil {
		_, err := tx.Exec(`
			INSERT INTO model_ratings (model_id, category_id, rating, wins, losses, draws, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(model_id, category_id) DO UPDATE SET
				rating = rating + ? - model_ratings.rating,
				wins = wins + ?,
				losses = losses + ?,
				draws = draws + ?,
				updated_at = CURRENT_TIMESTAMP
		`, modelID, *categoryID, rating, wins, losses, draws, rating, wins, losses, draws)
		return err
	}

	_, err := tx.Exec(`
		INSERT INTO model_ratings (model_id, category_id, rating, wins, losses, draws, updated_at)
		VALUES (?, NULL, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(model_id, category_id) DO UPDATE SET
			rating = ?,
			wins = wins + ?,
			losses = losses + ?,
			draws = draws + ?,
			updated_at = CURRENT_TIMESTAMP
	`, modelID, rating, wins, losses, draws, rating, wins, losses, draws)
	return err
}

func (c *Calculator) recordHistory(tx *sql.Tx, modelID string, categoryID *int64, sessionID string, oldRating, newRating, change int) error {
	var reason string
	switch {
	case change > 0:
		reason = "win"
	case change < 0:
		reason = "loss"
	default:
		reason = "draw"
	}

	_, err := tx.Exec(`
		INSERT INTO elo_history (model_id, category_id, session_id, old_rating, new_rating, change, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, modelID, categoryID, sessionID, oldRating, newRating, change, reason)
	return err
}

// UpdateMatchup updates the head-to-head record between two models
func (c *Calculator) UpdateMatchup(tx *sql.Tx, modelA, modelB string, categoryID *int64, winnerID string) error {
	// Ensure consistent ordering
	if modelA > modelB {
		modelA, modelB = modelB, modelA
	}

	var aWins, bWins, draws int
	switch winnerID {
	case modelA:
		aWins = 1
	case modelB:
		bWins = 1
	default:
		draws = 1
	}

	_, err := tx.Exec(`
		INSERT INTO matchups (model_a_id, model_b_id, category_id, model_a_wins, model_b_wins, draws, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(model_a_id, model_b_id, category_id) DO UPDATE SET
			model_a_wins = model_a_wins + ?,
			model_b_wins = model_b_wins + ?,
			draws = draws + ?,
			updated_at = CURRENT_TIMESTAMP
	`, modelA, modelB, categoryID, aWins, bWins, draws, aWins, bWins, draws)
	return err
}

// GetModelStats returns comprehensive stats for a model
type ModelStats struct {
	ModelID    string `json:"model_id"`
	Rating     int    `json:"rating"`
	Wins       int    `json:"wins"`
	Losses     int    `json:"losses"`
	Draws      int    `json:"draws"`
	WinRate    float64 `json:"win_rate"`
	GamesPlayed int   `json:"games_played"`
}

func (c *Calculator) GetModelStats(modelID string, categoryID *int64) (*ModelStats, error) {
	rating, games, err := c.getModelRating(modelID, categoryID)
	if err != nil {
		return nil, err
	}

	var wins, losses, draws int
	var query string
	var args []interface{}

	if categoryID != nil {
		query = `SELECT COALESCE(wins, 0), COALESCE(losses, 0), COALESCE(draws, 0)
				 FROM model_ratings WHERE model_id = ? AND category_id = ?`
		args = []interface{}{modelID, *categoryID}
	} else {
		query = `SELECT COALESCE(SUM(wins), 0), COALESCE(SUM(losses), 0), COALESCE(SUM(draws), 0)
				 FROM model_ratings WHERE model_id = ?`
		args = []interface{}{modelID}
	}

	err = c.db.QueryRow(query, args...).Scan(&wins, &losses, &draws)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	winRate := 0.0
	if games > 0 {
		winRate = float64(wins) / float64(games)
	}

	return &ModelStats{
		ModelID:     modelID,
		Rating:      rating,
		Wins:        wins,
		Losses:      losses,
		Draws:       draws,
		WinRate:     winRate,
		GamesPlayed: games,
	}, nil
}
