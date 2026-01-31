package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/sainaif/council/internal/services/auth"
)

type AuthMiddleware struct {
	secretKey string
}

func NewAuthMiddleware(secretKey string) *AuthMiddleware {
	return &AuthMiddleware{secretKey: secretKey}
}

func (m *AuthMiddleware) Required() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := m.extractClaims(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   true,
				"message": "Unauthorized",
			})
		}

		// Store claims in context
		c.Locals("user", claims)
		c.Locals("userID", claims.UserID)
		c.Locals("username", claims.Username)

		return c.Next()
	}
}

func (m *AuthMiddleware) Optional() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := m.extractClaims(c)
		if err == nil {
			c.Locals("user", claims)
			c.Locals("userID", claims.UserID)
			c.Locals("username", claims.Username)
		}
		return c.Next()
	}
}

func (m *AuthMiddleware) extractClaims(c *fiber.Ctx) (*auth.Claims, error) {
	// Try Authorization header first
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return m.parseToken(parts[1])
		}
	}

	// Try cookie
	tokenCookie := c.Cookies("council_token")
	if tokenCookie != "" {
		return m.parseToken(tokenCookie)
	}

	return nil, fiber.NewError(fiber.StatusUnauthorized, "No token provided")
}

func (m *AuthMiddleware) parseToken(tokenString string) (*auth.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &auth.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid token signing method")
		}
		return []byte(m.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*auth.Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
}

// GetUserID returns the user ID from context
func GetUserID(c *fiber.Ctx) string {
	if userID, ok := c.Locals("userID").(string); ok {
		return userID
	}
	return ""
}

// GetUsername returns the username from context
func GetUsername(c *fiber.Ctx) string {
	if username, ok := c.Locals("username").(string); ok {
		return username
	}
	return ""
}

// GetClaims returns the full claims from context
func GetClaims(c *fiber.Ctx) *auth.Claims {
	if claims, ok := c.Locals("user").(*auth.Claims); ok {
		return claims
	}
	return nil
}
