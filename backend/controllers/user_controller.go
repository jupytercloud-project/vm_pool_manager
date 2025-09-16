package controllers

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"

	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func GetUsers(c *gin.Context) {
	var users []models.User
	result := config.Database.Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func CreateUser(db *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var input models.User
		if err := ctx.ShouldBindJSON(&input); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		hashpass, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 14)
		input.Password = string(hashpass)

		if err := db.Create(&input).Error; err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		ctx.JSON(http.StatusCreated, input)
	}
}

func GetProfile(c *gin.Context) {
	userID, exist := c.Get("user_id")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
		return
	}
	var user models.User
	if err := config.Database.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "your profile (protected)",
		"id":      user.ID,
		"name":    user.Name,
		"email":   user.Email,
	})
}

func DeleteUser(c *gin.Context) {
	userID, exist := c.Get("user_id")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
		return
	}

	var user models.User
	if err := config.Database.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	config.Database.Delete(&user)
	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}
