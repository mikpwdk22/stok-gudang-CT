package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ctstock/backend/internal/models"
	"ctstock/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo  *repository.UserRepository
	jwtSecret []byte
}

type JWTClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	jwt.RegisteredClaims
}

func NewAuthService(userRepo *repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: []byte(jwtSecret),
	}
}

// Login verifies credentials and returns a JWT token with user details
func (s *AuthService) Login(ctx context.Context, username, password string) (string, models.UserResponse, error) {
	if username == "" || password == "" {
		return "", models.UserResponse{}, errors.New("username and password are required")
	}

	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return "", models.UserResponse{}, err
	}
	if user == nil {
		return "", models.UserResponse{}, errors.New("invalid username or password")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", models.UserResponse{}, errors.New("invalid username or password")
	}

	// Generate JWT token valid for 7 days
	claims := JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		FullName: user.FullName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ctstock-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", models.UserResponse{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, user.ToResponse(), nil
}

// ValidateToken parses and verifies a JWT token
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// GetUserByID gets a user by ID
func (s *AuthService) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

