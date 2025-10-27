package controllers

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/pb"
	"PoolManagerVM/backend/utils"
	"context"
	"encoding/json"
	"errors"
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

// helper pour sérialiser en JSON dans la map de réponse protobuf
func marshalToStringMap(key string, v interface{}) (map[string]string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return map[string]string{key: string(b)}, nil
}

// GetAllFlavorsRPC -> renvoie la liste triée des flavors
func GetAllFlavorsRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	var flavors []models.Flavor
	if err := config.Database.Find(&flavors).Error; err != nil {
		return nil, err
	}

	sort.Slice(flavors, func(i, j int) bool {
		return flavors[i].Name < flavors[j].Name
	})

	m, err := marshalToStringMap("flavors", flavors)
	if err != nil {
		return nil, err
	}

	return &pb.RessourceResponse{
		Userid: req.GetUserid(),
		Data:   m,
	}, nil
}

// GetAllNetworksRPC -> renvoie la liste triée des networks
func GetAllNetworksRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	var nets []models.Network
	if err := config.Database.Find(&nets).Error; err != nil {
		return nil, err
	}

	sort.Slice(nets, func(i, j int) bool {
		return nets[i].Name < nets[j].Name
	})

	m, err := marshalToStringMap("networks", nets)
	if err != nil {
		return nil, err
	}

	return &pb.RessourceResponse{
		Userid: req.GetUserid(),
		Data:   m,
	}, nil
}

// GetGroupeImageRPC -> filtre les images par groupe (req.Data["group"])
func GetGroupeImageRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	group, ok := req.GetData()["group"]
	if !ok || strings.TrimSpace(group) == "" {
		return nil, errors.New("missing group in request data")
	}

	var images []models.Image
	if err := config.Database.Find(&images).Error; err != nil {
		return nil, err
	}

	var filtered []models.Image
	for _, img := range images {
		named := strings.ToLower(utils.FirstLetters(img.Name))
		if named == strings.ToLower(group) {
			filtered = append(filtered, img)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	m, err := marshalToStringMap("images", filtered)
	if err != nil {
		return nil, err
	}

	return &pb.RessourceResponse{
		Userid: req.GetUserid(),
		Data:   m,
	}, nil
}

// GetGroupeImagenameRPC -> renvoie la liste des groupes disponibles {name,value}
func GetGroupeImagenameRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	groupMap := make(map[string][]string)
	var images []models.Image

	if err := config.Database.Find(&images).Error; err != nil {
		return nil, err
	}

	for _, img := range images {
		named := strings.ToLower(utils.FirstLetters(img.Name))
		groupMap[named] = append(groupMap[named], img.Name)
	}

	type GroupeImageName struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	var groupList []GroupeImageName
	for k := range groupMap {
		groupList = append(groupList, GroupeImageName{Name: k, Value: k})
	}

	sort.Slice(groupList, func(i, j int) bool {
		return groupList[i].Name < groupList[j].Name
	})

	m, err := marshalToStringMap("groups", groupList)
	if err != nil {
		return nil, err
	}

	return &pb.RessourceResponse{
		Userid: req.GetUserid(),
		Data:   m,
	}, nil
}
