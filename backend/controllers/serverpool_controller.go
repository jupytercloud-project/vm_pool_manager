package controllers

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/internal/worker"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/utils"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func GetServerpool(c *gin.Context) {

	allServers, err := utils.GetAllServers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}

	var activeServs []gin.H
	for _, s := range allServers {
		activeServs = append(activeServs, gin.H{
			"id":       s.ID,
			"name":     s.Name,
			"HostID":   s.HostID,
			"status":   s.Status,
			"Progress": s.Progress,
		})
	}
	c.JSON(http.StatusOK, gin.H{"servers": activeServs})
}

func CreateServerpool(c *gin.Context) {
	//essai avec les meme image et flavor que admin
	userID, exist := c.Get("user_id")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
		return
	}

	var body struct {
		Namesp string `json:"namesp"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.Database.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := config.Database.Create(&models.Serverpool{
		UserID:       user.Email,
		ServerpoolID: body.Namesp,
		ImageRef:     os.Getenv("SERVER_IMAGE_REF"),
		FlavorRef:    os.Getenv("SERVER_FLAVOR_REF"),
		Networks:     models.JSONStringSlice{os.Getenv("NETWORK_ID")},
		MinVM:        2,
		MaxVM:        4,
		PendingJobs:  0,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create serverpool"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "serverpool created",
		"serverpool": body.Namesp,
	})
}

func DeleteServerpool(c *gin.Context) {
	userID, exist := c.Get("user_id")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
		return
	}

	var body struct {
		Namesp string `json:"namesp"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.Database.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := config.Database.Where("user_id = ? AND serverpool_id = ?", user.Email, body.Namesp).
		Delete(&models.Serverpool{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot delete serverpool"})
		return
	}

	// pour tout les servers qui ont la paire user.email et body.namesp, creer un job highpriority pour les delete de openstack
	allServers, err := utils.GetAllServers()
	if err != nil {
		return
	}

	for _, ops := range allServers {
		s := models.FromGopherServer(ops)
		if s.UserID == user.Email && s.ServerpoolID == body.Namesp {
			var args []string
			args = append(args, "instance_id")
			args = append(args, s.ID)
			worker.AddJob(*worker.CreateJob(worker.DeleteVM, utils.BuildDataMap(args)), true)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "serverpool deleted",
		"serverpool": body.Namesp,
	})
}
