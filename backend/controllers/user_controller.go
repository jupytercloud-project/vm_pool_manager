package controllers

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"

	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// return a list of all users (not useful)
func GetUsers(c *gin.Context) {
	var users []models.User
	result := config.Database.Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

// Create a user in the database
// takes name, email and password
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

func GetUserConfigs(c *gin.Context) {
	userID, exist := c.Get("email")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
		return
	}

	var configs []models.ConfigPool
	if err := config.Database.Where("user_id = ?", userID).Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, configs)
}

func CreateUserConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exist := c.Get("email")
		if !exist {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
			return
		}

		var input struct {
			Name string `json:"name"`
			Data string `json:"data"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		configPool := models.ConfigPool{
			UserID: userID.(string),
			Name:   input.Name,
			Data:   input.Data,
		}

		if err := db.Create(&configPool).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, configPool)
	}
}

func DeleteUserConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exist := c.Get("user_id")
		if !exist {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
			return
		}

		configID := c.Param("config_id")
		var configPool models.ConfigPool
		if err := db.First(&configPool, configID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}

		if configPool.UserID != userID.(string) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission to delete this config"})
			return
		}

		db.Delete(&configPool)
		c.JSON(http.StatusOK, gin.H{"message": "config deleted"})
	}
}

func UpdateUserConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exist := c.Get("user_id")
		if !exist {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
			return
		}

		configID := c.Param("config_id")
		var configPool models.ConfigPool
		if err := db.First(&configPool, configID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
			return
		}

		if configPool.UserID != userID.(string) {
			c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission to update this config"})
			return
		}

		var input struct {
			Name string `json:"name"`
			Data string `json:"data"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		configPool.Name = input.Name
		configPool.Data = input.Data

		if err := db.Save(&configPool).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, configPool)
	}
}
