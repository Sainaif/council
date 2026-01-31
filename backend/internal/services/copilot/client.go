package copilot

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Model represents an available AI model
type Model struct {
	ID           string   `json:"id"`
	DisplayName  string   `json:"display_name"`
	Provider     string   `json:"provider"`
	Capabilities []string `json:"capabilities"`
}

// Response represents a model response
type Response struct {
	Content      string `json:"content"`
	TokenCount   int    `json:"token_count"`
	ResponseTime int64  `json:"response_time_ms"`
	Error        error  `json:"error,omitempty"`
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	Content    string `json:"content"`
	Done       bool   `json:"done"`
	TokenCount int    `json:"token_count,omitempty"`
	Error      error  `json:"error,omitempty"`
}

// Service manages Copilot SDK interactions
type Service struct {
	models   []Model
	cacheTTL time.Duration
	mu       sync.RWMutex
	shutdown chan struct{}
}

// NewService creates a new Copilot service
func NewService() *Service {
	s := &Service{
		cacheTTL: 5 * time.Minute,
		shutdown: make(chan struct{}),
	}

	// Initialize with default models (these would come from Copilot SDK in production)
	s.models = getDefaultModels()

	return s
}

// getDefaultModels returns a list of available models
// In production, this would query the Copilot SDK
func getDefaultModels() []Model {
	return []Model{
		{ID: "gpt-4o", DisplayName: "GPT-4o", Provider: "openai", Capabilities: []string{"chat", "code", "reasoning"}},
		{ID: "gpt-4o-mini", DisplayName: "GPT-4o Mini", Provider: "openai", Capabilities: []string{"chat", "code"}},
		{ID: "claude-sonnet-4", DisplayName: "Claude Sonnet 4", Provider: "anthropic", Capabilities: []string{"chat", "code", "reasoning"}},
		{ID: "claude-3.5-sonnet", DisplayName: "Claude 3.5 Sonnet", Provider: "anthropic", Capabilities: []string{"chat", "code", "reasoning"}},
		{ID: "gemini-2.0-flash", DisplayName: "Gemini 2.0 Flash", Provider: "google", Capabilities: []string{"chat", "code"}},
		{ID: "o1", DisplayName: "o1", Provider: "openai", Capabilities: []string{"reasoning", "code"}},
		{ID: "o1-mini", DisplayName: "o1 Mini", Provider: "openai", Capabilities: []string{"reasoning"}},
		{ID: "o3-mini", DisplayName: "o3 Mini", Provider: "openai", Capabilities: []string{"reasoning", "code"}},
	}
}

// ListModels returns available models
func (s *Service) ListModels(ctx context.Context) ([]Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.models, nil
}

// GetModel returns a specific model by ID
func (s *Service) GetModel(ctx context.Context, id string) (*Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.models {
		if m.ID == id {
			return &m, nil
		}
	}

	return nil, fmt.Errorf("model not found: %s", id)
}

// SendPrompt sends a prompt to a model and returns the full response
func (s *Service) SendPrompt(ctx context.Context, modelID, prompt string) (*Response, error) {
	log.Printf("[COPILOT] SendPrompt - model: %s, prompt length: %d chars", modelID, len(prompt))
	start := time.Now()

	// TODO: Integrate with actual Copilot SDK
	// For now, return a placeholder response
	response := &Response{
		Content:      fmt.Sprintf("[Response from %s] This is a placeholder response. The Copilot SDK integration will provide actual model responses.", modelID),
		TokenCount:   100,
		ResponseTime: time.Since(start).Milliseconds(),
	}

	log.Printf("[COPILOT] SendPrompt completed - model: %s, response time: %dms", modelID, response.ResponseTime)
	return response, nil
}

// StreamPrompt sends a prompt and streams the response
func (s *Service) StreamPrompt(ctx context.Context, modelID, prompt string) (<-chan StreamChunk, error) {
	log.Printf("[COPILOT] StreamPrompt - model: %s, prompt length: %d chars", modelID, len(prompt))
	chunks := make(chan StreamChunk, 100)

	go func() {
		defer close(chunks)

		// TODO: Integrate with actual Copilot SDK for streaming
		// For now, simulate streaming with placeholder
		words := []string{
			"[Response", "from", modelID + "]",
			"This", "is", "a", "placeholder", "streaming", "response.",
			"The", "Copilot", "SDK", "integration", "will", "provide",
			"actual", "model", "responses", "with", "real", "streaming.",
		}

		for i, word := range words {
			select {
			case <-ctx.Done():
				chunks <- StreamChunk{Error: ctx.Err()}
				return
			case <-s.shutdown:
				return
			default:
				chunks <- StreamChunk{
					Content: word + " ",
					Done:    i == len(words)-1,
				}
				time.Sleep(50 * time.Millisecond) // Simulate streaming delay
			}
		}

		// Final chunk with token count
		chunks <- StreamChunk{
			Done:       true,
			TokenCount: len(words) * 2,
		}
	}()

	return chunks, nil
}

// RequestVote asks a model to vote on anonymized responses
func (s *Service) RequestVote(ctx context.Context, modelID, question string, responses map[string]string) ([]string, error) {
	// Build voting prompt
	prompt := fmt.Sprintf(`You are evaluating responses to the following question:

Question: %s

Here are the anonymized responses:

`, question)

	for label, content := range responses {
		prompt += fmt.Sprintf("--- %s ---\n%s\n\n", label, content)
	}

	prompt += `Please rank these responses from best to worst. Return ONLY a comma-separated list of labels in order from best to worst (e.g., "Response B, Response A, Response C").`
	_ = prompt // Will be used when Copilot SDK is integrated

	// TODO: Integrate with actual Copilot SDK
	// For now, return a placeholder ranking
	labels := make([]string, 0, len(responses))
	for label := range responses {
		labels = append(labels, label)
	}

	return labels, nil
}

// RequestSynthesis asks the chairperson to synthesize responses
func (s *Service) RequestSynthesis(ctx context.Context, modelID, question string, responses map[string]string, votes map[string][]string) (*Response, error) {
	prompt := fmt.Sprintf(`You are the chairperson synthesizing a council discussion.

Original Question: %s

Council Responses:
`, question)

	for label, content := range responses {
		prompt += fmt.Sprintf("--- %s ---\n%s\n\n", label, content)
	}

	prompt += "\nVoting Results:\n"
	for voter, ranking := range votes {
		prompt += fmt.Sprintf("- %s ranked: %v\n", voter, ranking)
	}

	prompt += `\nPlease provide a comprehensive synthesis that:
1. Identifies the consensus view
2. Highlights key insights from top-ranked responses
3. Notes any significant minority opinions
4. Provides a clear, actionable answer to the original question`

	return s.SendPrompt(ctx, modelID, prompt)
}

// Shutdown gracefully shuts down the service
func (s *Service) Shutdown() {
	close(s.shutdown)
}

// IsModelAvailable checks if a model is available
func (s *Service) IsModelAvailable(modelID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.models {
		if m.ID == modelID {
			return true
		}
	}
	return false
}
