package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"

	"control_center/frontcontrolpb"
	oidchelper "control_center/internal/oidc"
	"control_center/models"
	"control_center/pb"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Service struct {
	frontcontrolpb.UnimplementedAuthServiceServer
	DB *gorm.DB
	pm pb.PoolManagerClient
}

func New(db *gorm.DB, pm pb.PoolManagerClient) *Service {
	return &Service{DB: db, pm: pm}
}

func (s *Service) CreateUser(
	ctx context.Context,
	req *frontcontrolpb.CreateUserRequest,
) (*frontcontrolpb.CreateUserResponse, error) {

	if req.Username == "" || req.Email == "" || req.Password == "" {
		return &frontcontrolpb.CreateUserResponse{Success: false}, fmt.Errorf("missing required fields")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return &frontcontrolpb.CreateUserResponse{Success: false}, err
	}

	role := "student"
	var count int64
	s.DB.Model(&models.User{}).Count(&count)
	if count == 0 {
		role = "admin"
	}

	u := models.User{
		Name:     req.Username,
		Email:    req.Email,
		Password: string(hashed),
		Role:     role,
	}
	if err := s.DB.Create(&u).Error; err != nil {
		return &frontcontrolpb.CreateUserResponse{Success: false}, fmt.Errorf("failed to create user: %v", err)
	}

	// Create user in GLAuth LDAP
	sha256hex := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Password)))
	if err := oidchelper.CreateLDAPUser(req.Username, req.Email, sha256hex, role == "admin"); err != nil {
		log.Printf("[auth] GLAuth user creation failed (non-fatal): %v", err)
	}

	_, err = s.pm.SendRessources(context.Background(), &pb.RessourceRequest{
		User: u.Email,
		Data: map[string]string{
			"name":  u.Name,
			"email": u.Email,
		},
		Status: pb.Status_CREATE,
		Type:   pb.Type_USER,
	})
	if err != nil {
		log.Printf("[auth] PoolManager sync failed (non-fatal): %v", err)
	}

	return &frontcontrolpb.CreateUserResponse{Success: true, UserId: fmt.Sprintf("%d", u.ID)}, nil
}

func (s *Service) AuthenticateUser(
	ctx context.Context,
	req *frontcontrolpb.AuthenticateUserRequest,
) (*frontcontrolpb.AuthenticateUserResponse, error) {
	var user models.User
	err := s.DB.Where("email = ?", req.Email).First(&user).Error
	if err != nil {
		return &frontcontrolpb.AuthenticateUserResponse{Success: false}, fmt.Errorf("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		// SÉCURITÉ : pas de fallback mot de passe en clair (un mot de passe stocké non hashé
		// ne doit jamais être accepté). Seul le hash bcrypt fait foi.
		return &frontcontrolpb.AuthenticateUserResponse{Success: false}, fmt.Errorf("invalid password")
	}

	token := fmt.Sprintf("%s:%s:%d", user.Role, user.Email, user.ID)
	return &frontcontrolpb.AuthenticateUserResponse{Success: true, Token: token}, nil
}
