package controllers

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/pb"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Endpoint to login
// takes email and password
// return JWT in body
func LoginUser(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	config.DBmu.Lock()
	if err := config.Database.Where("email = ?", input.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		config.DBmu.Unlock()
		return
	}
	config.DBmu.Unlock()

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Password"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"name":    user.Name,
		"email":   user.Email,
		"exp":     time.Now().Add(time.Hour * 4).Unix(),
	})

	tokerString, err := token.SignedString(config.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "logged in",
		"token":   tokerString,
	})
}

// AuthenticateForRPC vérifie les identifiants et renvoie un JWT
func AuthenticateForRPC(ctx context.Context, email, password string) (string, *models.User, error) {
	if email == "" || password == "" {
		return "", nil, errors.New("missing credentials")
	}

	var user models.User
	config.DBmu.Lock()
	err := config.Database.Where("email = ?", email).First(&user).Error
	config.DBmu.Unlock()
	if err != nil {
		return "", nil, errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", nil, errors.New("invalid password")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"name":    user.Name,
		"email":   user.Email,
		"exp":     time.Now().Add(4 * time.Hour).Unix(),
	})

	tokerString, err := token.SignedString(config.JWTSecret)
	if err != nil {
		return "", nil, errors.New("cannot generate token")
	}

	return tokerString, &user, nil
}

// LoginRPC -> wrapper compatible gRPC qui reçoit email/password via req.Data
// attend req.Data["email"] et req.Data["password"]
// renvoie dans RessourceResponse.Data["token"] et RessourceResponse.Userid = user.ID
func LoginRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	email := ""
	password := ""
	if req != nil && req.GetData() != nil {
		if v, ok := req.GetData()["email"]; ok {
			email = v
		}
		if v, ok := req.GetData()["password"]; ok {
			password = v
		}
	}

	token, user, err := AuthenticateForRPC(ctx, email, password)
	if err != nil {
		return nil, err
	}

	resp := &pb.RessourceResponse{
		Userid: user.Email,
		Data:   map[string]string{"token": token},
	}
	return resp, nil
}
