package controllers

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/utils"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetallFlavors(c *gin.Context) {
	var flavor []models.Flavor
	if err := config.Database.Find(&flavor).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch flavors"})
		return
	}

	sort.Slice(flavor, func(i, j int) bool {
		return flavor[i].Name < flavor[j].Name
	})

	c.JSON(http.StatusOK, flavor)
}

func GetAllNetworks(c *gin.Context) {
	var networks []models.Network
	if err := config.Database.Find(&networks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch networks"})
		return
	}

	sort.Slice(networks, func(i, j int) bool {
		return networks[i].Name < networks[j].Name
	})

	c.JSON(http.StatusOK, networks)
}

type GroupRequest struct {
	Group string `json:"group"`
}

func GetGroupeImage(c *gin.Context) {
	var req GroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var images []models.Image
	if err := config.Database.Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch images"})
		return
	}

	var filtered []models.Image
	for _, img := range images {
		named := strings.ToLower(utils.FirstLetters(img.Name))
		if named == strings.ToLower(req.Group) {
			filtered = append(filtered, img)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	c.JSON(http.StatusOK, filtered)
}

type GroupeImageName struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func GetGroupeImagename(c *gin.Context) {
	groupMap := make(map[string][]string)
	var images []models.Image

	if err := config.Database.Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch images"})
		return
	}

	for _, img := range images {
		named := strings.ToLower(utils.FirstLetters(img.Name))
		groupMap[named] = append(groupMap[named], img.Name)
	}

	var groupList []GroupeImageName
	for k := range groupMap {
		groupList = append(groupList, GroupeImageName{Name: k, Value: k})
	}

	sort.Slice(groupList, func(i, j int) bool {
		return groupList[i].Name < groupList[j].Name
	})

	c.JSON(http.StatusOK, groupList)
}
