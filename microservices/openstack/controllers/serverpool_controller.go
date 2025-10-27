package controllers

import (
	"PoolManagerVM/backend/config"
	"PoolManagerVM/backend/internal/worker"
	"PoolManagerVM/backend/models"
	"PoolManagerVM/backend/pb"
	"PoolManagerVM/backend/utils"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/utils/v2/openstack/clientconfig"
)

// create a serverpool in DB, instances will be created by maincrawler
// take only the name of the new serverpool, with authentication before
// create serverpool with base config for now, adding possibles configuration from form
func CreateServerpool(c *gin.Context) {
	//essai avec les meme image et flavor que admin
	userID, exist := c.Get("user_id")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
		return
	}

	var body struct {
		Namesp      string   `json:"namesp"`
		ImageRef    string   `json:"image_ref"`
		FlavorRef   string   `json:"flavor_ref"`
		Networks    []string `json:"networks"`
		MinVM       int      `json:"min_vm"`
		MaxVM       int      `json:"max_vm"`
		Config_file int      `json:"config_file"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	config.DBmu.Lock()
	if err := config.Database.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		config.DBmu.Unlock()
		return
	}

	if err := config.Database.Create(&models.Serverpool{
		UserID:       user.Email,
		ServerpoolID: body.Namesp,
		ImageRef:     body.ImageRef,
		FlavorRef:    body.FlavorRef,
		Networks:     models.JSONStringSlice(body.Networks),
		MinVM:        body.MinVM,
		MaxVM:        body.MaxVM,
		ConfigID:     body.Config_file,
		PendingJobs:  0,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create serverpool"})
		config.DBmu.Unlock()
		return
	}
	config.DBmu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message":    "serverpool created",
		"serverpool": body.Namesp,
	})
}

// delete a serverpool in DB and lauching jobs to delete instances
// takes only serverpool_ID and need to be authenticated
func DeleteServerpool(c *gin.Context) {
	userID, exist := c.Get("user_id")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
		return
	}

	serverpoolID := c.Param("id")
	if serverpoolID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing serverpool_id"})
		return
	}

	var user models.User
	config.DBmu.Lock()
	if err := config.Database.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		config.DBmu.Unlock()
		return
	}

	var sp models.Serverpool

	if err := config.Database.First(&sp, "user_id = ? AND serverpool_id = ?", user.Email, serverpoolID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "serverpool not found"})
		config.DBmu.Unlock()
		return
	}

	if err := config.Database.Delete(&sp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot delete serverpool"})
		config.DBmu.Unlock()
		return
	}
	config.DBmu.Unlock()

	// pour tout les servers qui ont la paire user.email et body.namesp, creer un job highpriority pour les delete de openstack
	allServers, err := utils.GetAllServers()
	if err != nil {
		return
	}

	for _, ops := range allServers {
		s := models.FromGopherServer(ops)
		if s.UserID == user.Email && s.ServerpoolID == serverpoolID {
			var args []string
			args = append(args, "instance_id")
			args = append(args, s.ID)
			worker.AddJob(*worker.CreateJob(models.DeleteVM, utils.BuildDataMap(args)), true)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "serverpool deleted",
		"serverpool": serverpoolID,
	})
}

func GetMyServerpools(c *gin.Context) {
	userID, exist := c.Get("email")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
		return
	}
	allsp, err := utils.GetAllServerPool()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve serverpools from Openstack"})
		return
	}

	var ressps []gin.H

	for _, sp := range allsp {
		if sp.UserID == userID {
			ressps = append(ressps, gin.H{
				"serverpool_id": sp.ServerpoolID,
				"image_ref":     sp.ImageRef,
				"flavor_ref":    sp.FlavorRef,
				"networks":      sp.Networks,
				"min_vm":        sp.MinVM,
				"max_vm":        sp.MaxVM,
				"pending_jobs":  sp.PendingJobs,
				"config_file":   sp.ConfigID,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"serverpools": ressps})
}

func GetServersInServerpool(c *gin.Context) {
	userEmail, exist := c.Get("email")
	if !exist {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not connected"})
		return
	}

	serverpoolID := c.Param("id")
	if serverpoolID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing serverpool_id"})
		return
	}

	allServers, err := utils.GetAllServers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrieve servers"})
		return
	}

	var serversInPool []gin.H
	for _, s := range allServers {
		ms := models.FromGopherServer(s)
		if ms.UserID == userEmail && ms.ServerpoolID == serverpoolID {
			serversInPool = append(serversInPool, gin.H{
				"id":     s.ID,
				"name":   s.Name,
				"status": s.Status,
				"flavor": gin.H{
					"id":   s.Flavor["id"],
					"name": s.Flavor["name"],
				},
				"image": gin.H{
					"id":   s.Image["id"],
					"name": s.Image["name"],
				},
				"addresses": s.Addresses,
				"created":   s.Created,
				"updated":   s.Updated,
				"host_id":   s.HostID,
				"progress":  s.Progress,
				"config_id": ms.ConfigID,
			})

		}
	}
	// fmt.Println("SERVERS IN POOL:", serversInPool)
	c.JSON(http.StatusOK, gin.H{"servers": serversInPool})
}

type RebuildRequest struct {
	ServerID   string `json:"serverId" binding:"required"`
	ServerName string `json:"server_name" binding:"required"`
	ImageID    string `json:"image_id" binding:"required"`
}

func RebuildServer(c *gin.Context) {
	var req RebuildRequest

	// Lire le JSON du body
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Récupérer user_id / email injectés par le middleware
	userID, _ := c.Get("user_id")
	email, _ := c.Get("email")

	// Créer un client Compute via clouds.yaml
	opts := &clientconfig.ClientOpts{
		Cloud: os.Getenv("OPTS_CLOUD"), // ex: "devstack", "ovh", etc.
	}

	client, err := clientconfig.NewServiceClient(c, "compute", opts)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "failed to create compute client",
			"details": err.Error(),
		})
		return
	}

	// Préparer les options de rebuild
	rebuildOpts := servers.RebuildOpts{
		ImageRef: req.ImageID,
		Name:     req.ServerName,
	}

	// Exécuter le rebuild
	_, err = servers.Rebuild(c.Request.Context(), client, req.ServerID, rebuildOpts).Extract()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to rebuild server",
			"details": err.Error(),
		})
		return
	}

	// Réponse au frontend
	c.JSON(http.StatusOK, gin.H{
		"message":     "rebuild launched successfully",
		"server_id":   req.ServerID,
		"server_name": req.ServerName,
		"image_id":    req.ImageID,
		"user_id":     userID,
		"email":       email,
	})
}

// helper parse networks string: accept JSON array or comma separated
func parseNetworks(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}, nil
	}
	// try JSON array
	var asJson []string
	if strings.HasPrefix(s, "[") {
		if err := json.Unmarshal([]byte(s), &asJson); err != nil {
			return nil, err
		}
		return asJson, nil
	}
	// fallback comma separated
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			res = append(res, p)
		}
	}
	return res, nil
}

// CreateServerpoolRPC crée un serverpool pour l'utilisateur (req.Userid peut être user.ID ou email)
// attend dans req.Data : namesp, image_ref, flavor_ref, networks (json array ou csv), min_vm, max_vm, config_file (optional)
func CreateServerpoolRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	userIdentifier := req.GetUserid()
	if userIdentifier == "" {
		return nil, errors.New("missing userid")
	}

	// find user by id or email
	var user models.User
	config.DBmu.Lock()
	err := config.Database.First(&user, userIdentifier).Error
	if err != nil {
		// try by email
		err = config.Database.Where("email = ?", userIdentifier).First(&user).Error
		if err != nil {
			config.DBmu.Unlock()
			return nil, errors.New("user not found")
		}
	}
	// parse body fields
	data := req.GetData()
	namesp := data["namesp"]
	imageRef := data["image_ref"]
	flavorRef := data["flavor_ref"]
	networksStr := data["networks"]
	minVM := 0
	maxVM := 0
	if v, ok := data["min_vm"]; ok && v != "" {
		_ = json.Unmarshal([]byte(v), &minVM) // tolerate number as json string
	}
	if v, ok := data["max_vm"]; ok && v != "" {
		_ = json.Unmarshal([]byte(v), &maxVM)
	}
	configFile := 0
	if v, ok := data["config_file"]; ok && v != "" {
		_ = json.Unmarshal([]byte(v), &configFile)
	}

	networks, err := parseNetworks(networksStr)
	if err != nil {
		config.DBmu.Unlock()
		return nil, err
	}

	sp := &models.Serverpool{
		UserID:       user.Email,
		ServerpoolID: namesp,
		ImageRef:     imageRef,
		FlavorRef:    flavorRef,
		Networks:     models.JSONStringSlice(networks),
		MinVM:        minVM,
		MaxVM:        maxVM,
		ConfigID:     configFile,
		PendingJobs:  0,
	}

	if err := config.Database.Create(sp).Error; err != nil {
		config.DBmu.Unlock()
		return nil, err
	}
	config.DBmu.Unlock()

	respData := map[string]string{"serverpool": sp.ServerpoolID}
	return &pb.RessourceResponse{Userid: user.Email, Data: respData}, nil
}

// DeleteServerpoolRPC supprime un serverpool et queue des jobs de suppression des instances
// attend req.Data["serverpool_id"] (ou "id")
func DeleteServerpoolRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	userIdentifier := req.GetUserid()
	if userIdentifier == "" {
		return nil, errors.New("missing userid")
	}

	var user models.User
	config.DBmu.Lock()
	err := config.Database.First(&user, userIdentifier).Error
	if err != nil {
		err = config.Database.Where("email = ?", userIdentifier).First(&user).Error
		if err != nil {
			config.DBmu.Unlock()
			return nil, errors.New("user not found")
		}
	}

	data := req.GetData()
	serverpoolID := data["serverpool_id"]
	if serverpoolID == "" {
		serverpoolID = data["id"]
	}
	if serverpoolID == "" {
		config.DBmu.Unlock()
		return nil, errors.New("missing serverpool_id")
	}

	var sp models.Serverpool
	if err := config.Database.First(&sp, "user_id = ? AND serverpool_id = ?", user.Email, serverpoolID).Error; err != nil {
		config.DBmu.Unlock()
		return nil, errors.New("serverpool not found")
	}

	if err := config.Database.Delete(&sp).Error; err != nil {
		config.DBmu.Unlock()
		return nil, err
	}
	config.DBmu.Unlock()

	// queue delete jobs for matching servers
	allServers, err := utils.GetAllServers()
	if err == nil {
		for _, ops := range allServers {
			s := models.FromGopherServer(ops)
			if s.UserID == user.Email && s.ServerpoolID == serverpoolID {
				var args []string
				args = append(args, "instance_id")
				args = append(args, s.ID)
				worker.AddJob(*worker.CreateJob(models.DeleteVM, utils.BuildDataMap(args)), true)
			}
		}
	}

	resp := &pb.RessourceResponse{
		Userid: user.Email,
		Data:   map[string]string{"serverpool": serverpoolID, "message": "deleted"},
	}
	return resp, nil
}

// GetMyServerpoolsRPC retourne les serverpools de l'utilisateur
func GetMyServerpoolsRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	userIdentifier := req.GetUserid()
	if userIdentifier == "" {
		return nil, errors.New("missing userid")
	}

	var user models.User
	// try by id then by email
	config.DBmu.Lock()
	err := config.Database.First(&user, userIdentifier).Error
	if err != nil {
		err = config.Database.Where("email = ?", userIdentifier).First(&user).Error
		if err != nil {
			config.DBmu.Unlock()
			return nil, errors.New("user not found")
		}
	}
	config.DBmu.Unlock()

	allsp, err := utils.GetAllServerPool()
	if err != nil {
		return nil, err
	}

	type spOut struct {
		ServerpoolID string                 `json:"serverpool_id"`
		ImageRef     string                 `json:"image_ref"`
		FlavorRef    string                 `json:"flavor_ref"`
		Networks     models.JSONStringSlice `json:"networks"`
		MinVM        int                    `json:"min_vm"`
		MaxVM        int                    `json:"max_vm"`
		PendingJobs  int                    `json:"pending_jobs"`
		ConfigFile   int                    `json:"config_file"`
	}

	var ressps []spOut
	for _, sp := range allsp {
		if sp.UserID == user.Email {
			ressps = append(ressps, spOut{
				ServerpoolID: sp.ServerpoolID,
				ImageRef:     sp.ImageRef,
				FlavorRef:    sp.FlavorRef,
				Networks:     sp.Networks,
				MinVM:        sp.MinVM,
				MaxVM:        sp.MaxVM,
				PendingJobs:  sp.PendingJobs,
				ConfigFile:   sp.ConfigID,
			})
		}
	}

	b, err := json.Marshal(ressps)
	if err != nil {
		return nil, err
	}
	return &pb.RessourceResponse{Userid: user.Email, Data: map[string]string{"serverpools": string(b)}}, nil
}

// GetServersInServerpoolRPC retourne les serveurs d'un serverpool pour l'utilisateur
// attend req.Data["serverpool_id"] ou req.Data["id"]
func GetServersInServerpoolRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	userIdentifier := req.GetUserid()
	if userIdentifier == "" {
		return nil, errors.New("missing userid")
	}

	var user models.User
	config.DBmu.Lock()
	err := config.Database.First(&user, userIdentifier).Error
	if err != nil {
		err = config.Database.Where("email = ?", userIdentifier).First(&user).Error
		if err != nil {
			config.DBmu.Unlock()
			return nil, errors.New("user not found")
		}
	}
	config.DBmu.Unlock()

	data := req.GetData()
	serverpoolID := data["serverpool_id"]
	if serverpoolID == "" {
		serverpoolID = data["id"]
	}
	if serverpoolID == "" {
		return nil, errors.New("missing serverpool_id")
	}

	allServers, err := utils.GetAllServers()
	if err != nil {
		return nil, err
	}

	type outServer struct {
		ID        string                 `json:"id"`
		Name      string                 `json:"name"`
		Status    string                 `json:"status"`
		Flavor    map[string]interface{} `json:"flavor"`
		Image     map[string]interface{} `json:"image"`
		Addresses interface{}            `json:"addresses"`
		Created   string                 `json:"created"`
		Updated   string                 `json:"updated"`
		HostID    string                 `json:"host_id"`
		Progress  int                    `json:"progress"`
		ConfigID  int                    `json:"config_id"`
	}

	var serversInPool []outServer
	for _, s := range allServers {
		ms := models.FromGopherServer(s)
		if ms.UserID == user.Email && ms.ServerpoolID == serverpoolID {
			var flavorMap map[string]interface{}
			var imageMap map[string]interface{}
			if s.Flavor != nil {
				flavorMap = map[string]interface{}{
					"id":   s.Flavor["id"],
					"name": s.Flavor["name"],
				}
			}
			if s.Image != nil {
				imageMap = map[string]interface{}{
					"id":   s.Image["id"],
					"name": s.Image["name"],
				}
			}
			serversInPool = append(serversInPool, outServer{
				ID:        s.ID,
				Name:      s.Name,
				Status:    s.Status,
				Flavor:    flavorMap,
				Image:     imageMap,
				Addresses: s.Addresses,
				HostID:    s.HostID,
				Progress:  s.Progress,
				ConfigID:  ms.ConfigID,
			})
		}
	}

	b, err := json.Marshal(serversInPool)
	if err != nil {
		return nil, err
	}
	return &pb.RessourceResponse{Userid: user.Email, Data: map[string]string{"servers": string(b)}}, nil
}

// RebuildServerRPC -> lance un rebuild d'une instance
// attend req.Data["server_id"], req.Data["server_name"], req.Data["image_id"]
func RebuildServerRPC(ctx context.Context, req *pb.RessourceRequest) (*pb.RessourceResponse, error) {
	data := req.GetData()
	serverID := data["server_id"]
	if serverID == "" {
		serverID = data["serverId"]
	}
	serverName := data["server_name"]
	if serverName == "" {
		serverName = data["serverName"]
	}
	imageID := data["image_id"]
	if imageID == "" {
		imageID = data["imageId"]
	}
	if serverID == "" || serverName == "" || imageID == "" {
		return nil, errors.New("missing rebuild parameters")
	}

	// create compute client from clouds.yaml
	opts := &clientconfig.ClientOpts{
		Cloud: os.Getenv("OPTS_CLOUD"),
	}
	client, err := clientconfig.NewServiceClient(context.Background(), "compute", opts)
	if err != nil {
		return nil, err
	}

	rebuildOpts := servers.RebuildOpts{
		ImageRef: imageID,
		Name:     serverName,
	}

	_, err = servers.Rebuild(context.Background(), client, serverID, rebuildOpts).Extract()
	if err != nil {
		return nil, err
	}

	return &pb.RessourceResponse{Userid: req.GetUserid(), Data: map[string]string{
		"message":   "rebuild launched successfully",
		"server_id": serverID,
	}}, nil
}
