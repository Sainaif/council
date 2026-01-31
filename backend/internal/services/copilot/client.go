package copilot

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	copilot "github.com/github/copilot-sdk/go"
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

// userClient holds a Copilot client for a specific user
type userClient struct {
	client    *copilot.Client
	createdAt time.Time
	lastUsed  time.Time
}

// Service manages Copilot SDK interactions with per-user authentication
type Service struct {
	clients     map[string]*userClient // key: userID
	clientsMu   sync.RWMutex
	modelsCache map[string][]Model // key: userID
	modelsMu    sync.RWMutex
	cacheTTL    time.Duration
	shutdown    chan struct{}
	cleanupDone chan struct{}
}

// NewService creates a new Copilot service
func NewService() *Service {
	s := &Service{
		clients:     make(map[string]*userClient),
		modelsCache: make(map[string][]Model),
		cacheTTL:    5 * time.Minute,
		shutdown:    make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go s.cleanupLoop()

	return s
}

// cleanupLoop periodically cleans up idle clients
func (s *Service) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	defer close(s.cleanupDone)

	for {
		select {
		case <-ticker.C:
			s.cleanupIdleClients()
		case <-s.shutdown:
			return
		}
	}
}

// cleanupIdleClients removes clients that haven't been used recently
func (s *Service) cleanupIdleClients() {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	threshold := time.Now().Add(-30 * time.Minute)
	for userID, uc := range s.clients {
		if uc.lastUsed.Before(threshold) {
			log.Printf("[COPILOT] Cleaning up idle client for user: %s", userID)
			uc.client.Stop()
			delete(s.clients, userID)
		}
	}
}

// getOrCreateClient gets or creates a Copilot client for a user
func (s *Service) getOrCreateClient(userID, accessToken string) (*copilot.Client, error) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	// Check if client exists and is still valid
	if uc, exists := s.clients[userID]; exists {
		uc.lastUsed = time.Now()
		return uc.client, nil
	}

	// Create new client with user's token
	log.Printf("[COPILOT] Creating new client for user: %s", userID)

	opts := &copilot.ClientOptions{
		LogLevel:    "debug", // Enable debug to see what's happening
		AutoStart:   copilot.Bool(true),
		AutoRestart: copilot.Bool(true),
		GithubToken: accessToken,
	}

	client := copilot.NewClient(opts)

	// Start client with timeout
	startDone := make(chan error, 1)
	go func() {
		startDone <- client.Start()
	}()

	select {
	case err := <-startDone:
		if err != nil {
			log.Printf("[COPILOT] ERROR: Failed to start client for user %s: %v", userID, err)
			return nil, fmt.Errorf("failed to start Copilot client: %w", err)
		}
	case <-time.After(30 * time.Second):
		log.Printf("[COPILOT] ERROR: Timeout starting client for user %s", userID)
		client.Stop()
		return nil, fmt.Errorf("timeout starting Copilot client")
	}

	s.clients[userID] = &userClient{
		client:    client,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}

	log.Printf("[COPILOT] Client created successfully for user: %s", userID)
	return client, nil
}

// ListModels returns available models for a user (dynamically fetched from Copilot)
func (s *Service) ListModels(ctx context.Context, userID, accessToken string) ([]Model, error) {
	// Check cache first
	s.modelsMu.RLock()
	if cached, exists := s.modelsCache[userID]; exists {
		s.modelsMu.RUnlock()
		return cached, nil
	}
	s.modelsMu.RUnlock()

	// Get or create client
	client, err := s.getOrCreateClient(userID, accessToken)
	if err != nil {
		return nil, err
	}

	// Fetch models from SDK
	log.Printf("[COPILOT] Fetching models for user: %s", userID)
	modelInfos, err := client.ListModels()
	if err != nil {
		log.Printf("[COPILOT] ERROR: Failed to list models for user %s: %v", userID, err)
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	// Convert to our Model type
	models := make([]Model, 0, len(modelInfos))
	for _, m := range modelInfos {
		capabilities := []string{"chat"}
		// Check if this model likely supports reasoning based on name
		if contains(m.ID, "o1", "o3", "o4") {
			capabilities = append(capabilities, "reasoning")
		}

		models = append(models, Model{
			ID:           m.ID,
			DisplayName:  m.Name,
			Provider:     inferProvider(m.ID),
			Capabilities: capabilities,
		})
	}

	// Cache the results
	s.modelsMu.Lock()
	s.modelsCache[userID] = models
	s.modelsMu.Unlock()

	// Schedule cache expiration
	go func() {
		time.Sleep(s.cacheTTL)
		s.modelsMu.Lock()
		delete(s.modelsCache, userID)
		s.modelsMu.Unlock()
	}()

	log.Printf("[COPILOT] Loaded %d models for user: %s", len(models), userID)
	return models, nil
}

// inferProvider determines the provider from model ID
func inferProvider(modelID string) string {
	switch {
	case contains(modelID, "gpt", "o1", "o3", "o4"):
		return "openai"
	case contains(modelID, "claude"):
		return "anthropic"
	case contains(modelID, "gemini"):
		return "google"
	default:
		return "unknown"
	}
}

// contains checks if any of the substrings are in the string
func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// GetModel returns a specific model by ID
func (s *Service) GetModel(ctx context.Context, userID, accessToken, modelID string) (*Model, error) {
	models, err := s.ListModels(ctx, userID, accessToken)
	if err != nil {
		return nil, err
	}

	for _, m := range models {
		if m.ID == modelID {
			return &m, nil
		}
	}

	return nil, fmt.Errorf("model not found: %s", modelID)
}

// SendPrompt sends a prompt to a model and returns the full response
func (s *Service) SendPrompt(ctx context.Context, userID, accessToken, modelID, prompt string) (*Response, error) {
	log.Printf("[COPILOT] SendPrompt - user: %s, model: %s, prompt length: %d chars", userID, modelID, len(prompt))
	start := time.Now()

	client, err := s.getOrCreateClient(userID, accessToken)
	if err != nil {
		return nil, err
	}

	// Create a session for this request
	session, err := client.CreateSession(&copilot.SessionConfig{
		Model: modelID,
	})
	if err != nil {
		log.Printf("[COPILOT] ERROR: Failed to create session: %v", err)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer func() {
		if err := session.Destroy(); err != nil {
			log.Printf("[COPILOT] WARN: Failed to destroy session: %v", err)
		}
	}()

	// Send the message and wait for response
	resp, err := session.SendAndWait(copilot.MessageOptions{
		Prompt: prompt,
	}, time.Duration(120)*time.Second) // 2 minute timeout

	if err != nil {
		log.Printf("[COPILOT] ERROR: Failed to send prompt: %v", err)
		return nil, fmt.Errorf("failed to send prompt: %w", err)
	}

	content := ""
	if resp != nil && resp.Data.Content != nil {
		content = *resp.Data.Content
	}

	response := &Response{
		Content:      content,
		TokenCount:   estimateTokenCount(content),
		ResponseTime: time.Since(start).Milliseconds(),
	}

	log.Printf("[COPILOT] SendPrompt completed - user: %s, model: %s, response time: %dms, content length: %d",
		userID, modelID, response.ResponseTime, len(content))
	return response, nil
}

// StreamPrompt sends a prompt and streams the response
func (s *Service) StreamPrompt(ctx context.Context, userID, accessToken, modelID, prompt string) (<-chan StreamChunk, error) {
	log.Printf("[COPILOT] StreamPrompt - user: %s, model: %s, prompt length: %d chars", userID, modelID, len(prompt))
	chunks := make(chan StreamChunk, 100)

	client, err := s.getOrCreateClient(userID, accessToken)
	if err != nil {
		close(chunks)
		return nil, err
	}

	go func() {
		defer close(chunks)

		// Create a session with streaming enabled
		session, err := client.CreateSession(&copilot.SessionConfig{
			Model:     modelID,
			Streaming: true,
		})
		if err != nil {
			log.Printf("[COPILOT] ERROR: Failed to create streaming session: %v", err)
			chunks <- StreamChunk{Error: err}
			return
		}
		defer func() {
			if err := session.Destroy(); err != nil {
				log.Printf("[COPILOT] WARN: Failed to destroy streaming session: %v", err)
			}
		}()

		// Track content and completion
		var fullContent string
		done := make(chan struct{})

		// Subscribe to events
		unsubscribe := session.On(func(event copilot.SessionEvent) {
			select {
			case <-ctx.Done():
				return
			case <-s.shutdown:
				return
			default:
			}

			switch event.Type {
			case "assistant.message_delta":
				if event.Data.DeltaContent != nil {
					deltaContent := *event.Data.DeltaContent
					fullContent += deltaContent
					chunks <- StreamChunk{
						Content: deltaContent,
						Done:    false,
					}
				}
			case "assistant.message":
				// Final message received
				if event.Data.Content != nil {
					// Only send if we didn't already stream it
					if fullContent == "" {
						chunks <- StreamChunk{
							Content: *event.Data.Content,
							Done:    true,
						}
					}
				}
			case "session.idle":
				// Session finished processing
				close(done)
			case "session.error":
				errMsg := "session error"
				if event.Data.Message != nil {
					errMsg = *event.Data.Message
				}
				chunks <- StreamChunk{Error: fmt.Errorf("%s", errMsg)}
				close(done)
			}
		})
		defer unsubscribe()

		// Send the message
		_, err = session.Send(copilot.MessageOptions{
			Prompt: prompt,
		})
		if err != nil {
			log.Printf("[COPILOT] ERROR: Failed to send streaming prompt: %v", err)
			chunks <- StreamChunk{Error: err}
			return
		}

		// Wait for completion or context cancellation
		select {
		case <-done:
			// Final chunk with token count
			chunks <- StreamChunk{
				Done:       true,
				TokenCount: estimateTokenCount(fullContent),
			}
		case <-ctx.Done():
			if err := session.Abort(); err != nil {
				log.Printf("[COPILOT] WARN: Failed to abort session on context cancel: %v", err)
			}
			chunks <- StreamChunk{Error: ctx.Err()}
		case <-s.shutdown:
			if err := session.Abort(); err != nil {
				log.Printf("[COPILOT] WARN: Failed to abort session on shutdown: %v", err)
			}
		}
	}()

	return chunks, nil
}

// RequestVote asks a model to vote on anonymized responses
func (s *Service) RequestVote(ctx context.Context, userID, accessToken, modelID, question string, responses map[string]string) ([]string, error) {
	log.Printf("[COPILOT] RequestVote - user: %s, model: %s, responses: %d", userID, modelID, len(responses))

	// Build voting prompt
	prompt := fmt.Sprintf(`You are an expert evaluator assessing responses to a question. Your task is to rank the following anonymized responses from best to worst based on:
- Accuracy and correctness
- Completeness and depth
- Clarity and organization
- Practical usefulness

Question: %s

Here are the anonymized responses to evaluate:

`, question)

	labels := make([]string, 0, len(responses))
	for label, content := range responses {
		labels = append(labels, label)
		prompt += fmt.Sprintf("--- %s ---\n%s\n\n", label, content)
	}

	prompt += `Instructions:
1. Evaluate each response carefully
2. Return ONLY a comma-separated list of labels ranked from BEST to WORST
3. Example format: "Response B, Response A, Response C"
4. Do not include any other text, just the ranked list

Your ranking:`

	resp, err := s.SendPrompt(ctx, userID, accessToken, modelID, prompt)
	if err != nil {
		return nil, err
	}

	// Parse the response to extract rankings
	ranking := parseRanking(resp.Content, labels)
	if len(ranking) == 0 {
		// Fallback: return labels in original order
		log.Printf("[COPILOT] WARNING: Could not parse ranking, using original order")
		return labels, nil
	}

	log.Printf("[COPILOT] Vote result from %s: %v", modelID, ranking)
	return ranking, nil
}

// parseRanking extracts ranked labels from the response
func parseRanking(response string, validLabels []string) []string {
	var result []string
	seen := make(map[string]bool)

	// Look for labels in the response in order of appearance
	for _, label := range validLabels {
		for i := 0; i <= len(response)-len(label); i++ {
			if response[i:i+len(label)] == label && !seen[label] {
				// Check if it's a valid match (word boundary)
				validStart := i == 0 || !isAlphaNum(response[i-1])
				validEnd := i+len(label) >= len(response) || !isAlphaNum(response[i+len(label)])
				if validStart && validEnd {
					result = append(result, label)
					seen[label] = true
					break
				}
			}
		}
	}

	// Add any missing labels at the end
	for _, label := range validLabels {
		if !seen[label] {
			result = append(result, label)
		}
	}

	return result
}

func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// RequestSynthesis asks the chairperson to synthesize responses
func (s *Service) RequestSynthesis(ctx context.Context, userID, accessToken, modelID, question string, responses map[string]string, votes map[string][]string) (*Response, error) {
	log.Printf("[COPILOT] RequestSynthesis - user: %s, model: %s, responses: %d, voters: %d", userID, modelID, len(responses), len(votes))

	prompt := fmt.Sprintf(`You are the chairperson of an AI council. Your role is to synthesize the discussion and provide a comprehensive answer.

Original Question: %s

The council members have provided the following responses:

`, question)

	for label, content := range responses {
		prompt += fmt.Sprintf("--- %s ---\n%s\n\n", label, content)
	}

	prompt += "\nCouncil Voting Results (ranked from best to worst):\n"
	for voter, ranking := range votes {
		prompt += fmt.Sprintf("- %s ranked: %v\n", voter, ranking)
	}

	prompt += `

As the chairperson, please provide a synthesis that:
1. Identifies the consensus view based on voting results
2. Highlights key insights from the top-ranked responses
3. Notes any significant minority opinions or alternative perspectives
4. Provides a clear, comprehensive, and actionable final answer

Your synthesis:`

	return s.SendPrompt(ctx, userID, accessToken, modelID, prompt)
}

// Shutdown gracefully shuts down the service
func (s *Service) Shutdown() {
	log.Printf("[COPILOT] Shutting down Copilot service...")
	close(s.shutdown)

	// Wait for cleanup goroutine to finish
	<-s.cleanupDone

	// Stop all clients
	s.clientsMu.Lock()
	for userID, uc := range s.clients {
		log.Printf("[COPILOT] Stopping client for user: %s", userID)
		uc.client.Stop()
	}
	s.clients = make(map[string]*userClient)
	s.clientsMu.Unlock()

	log.Printf("[COPILOT] Copilot service shutdown complete")
}

// IsModelAvailable checks if a model is available for a user
func (s *Service) IsModelAvailable(ctx context.Context, userID, accessToken, modelID string) bool {
	models, err := s.ListModels(ctx, userID, accessToken)
	if err != nil {
		return false
	}

	for _, m := range models {
		if m.ID == modelID {
			return true
		}
	}
	return false
}

// estimateTokenCount provides a rough token estimate
func estimateTokenCount(content string) int {
	// Rough estimate: ~4 chars per token for English text
	return len(content) / 4
}
