package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/sainaif/council/internal/config"
)

type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type GitHubAuth struct {
	config      *oauth2.Config
	sessionKey  string
	tokenExpiry time.Duration
}

type Claims struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	AvatarURL   string `json:"avatar_url"`
	AccessToken string `json:"access_token"` // GitHub OAuth token for Copilot SDK
	jwt.RegisteredClaims
}

func NewGitHubAuth(cfg *config.Config) *GitHubAuth {
	return &GitHubAuth{
		config: &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			Scopes:       []string{"read:user", "user:email", "copilot"}, // copilot scope for Copilot SDK
			Endpoint:     github.Endpoint,
			RedirectURL:  cfg.OAuthCallbackURL(),
		},
		sessionKey:  cfg.SessionSecret,
		tokenExpiry: 24 * time.Hour * 7, // 7 days
	}
}

func (g *GitHubAuth) GetAuthURL(state string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (g *GitHubAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return g.config.Exchange(ctx, code)
}

func (g *GitHubAuth) GetUser(ctx context.Context, token *oauth2.Token) (*GitHubUser, error) {
	client := g.config.Client(ctx, token)

	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &user, nil
}

func (g *GitHubAuth) CreateToken(user *GitHubUser, accessToken string) (string, error) {
	claims := &Claims{
		UserID:      fmt.Sprintf("%d", user.ID),
		Username:    user.Login,
		AvatarURL:   user.AvatarURL,
		AccessToken: accessToken, // Store OAuth token for Copilot SDK
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(g.tokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "council-arena",
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(g.sessionKey))
}

func (g *GitHubAuth) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(g.sessionKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (g *GitHubAuth) GenerateState() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
