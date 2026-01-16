package attribvm

import (
	"bytes"
	"context"
	"control_center/frontcontrolpb"
	"control_center/models"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	frontcontrolpb.UnimplementedAttribVMServiceServer
	DB *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{
		DB: db,
	}
}

func (s *Service) ReturnPoolWithKey(
	req *frontcontrolpb.PoolWithKeyRequest,
	stream frontcontrolpb.AttribVMService_ReturnPoolWithKeyServer,
) error {
	pubKey := req.GetPubkey()
	if pubKey == "" {
		return status.Error(codes.InvalidArgument, "pubKey is empty")
	}

	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	var pools []models.Serverpool

	if err := s.DB.
		Where("keypublist @> ARRAY[?]::text[]", pubKey).
		Find(&pools).Error; err != nil {
		return status.Errorf(codes.Internal, "database error: %v", err)
	}

	for _, pool := range pools {
		resp := &frontcontrolpb.PoolWithKeyResponse{
			PoolId: pool.ServerpoolID,
			UserId: pool.UserID,
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) AttribVMinPool(
	ctx context.Context,
	req *frontcontrolpb.AttribVMinPoolRequest,
) (*frontcontrolpb.AttribVMinPoolResponse, error) {

	if req.GetServerpoolId() == "" || req.GetUserId() == "" || req.GetPubkey() == "" {
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     false,
			AddressedIp: "",
		}, status.Error(codes.InvalidArgument, "missing required fields")
	}

	var existingServer models.Server
	err := s.DB.
		Where("ssh_key_assigned = ? AND user_id = ? AND serverpool_id = ?", req.GetPubkey(), req.GetUserId(), req.GetServerpoolId()).
		First(&existingServer).Error

	if err == nil {
		log.Printf(
			"SSH key déjà associée → VM %s renvoyée",
			existingServer.IP_Address,
		)

		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     true,
			AddressedIp: existingServer.IP_Address,
		}, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success: false,
		}, err
	}

	var server models.Server
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("serverpool_id = ? AND user_id = ? AND locked = false",
				req.GetServerpoolId(), req.GetUserId()).
			Order("id").
			First(&server).Error; err != nil {

			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.ResourceExhausted, "no available server")
			}
			return err
		}
		if err := tx.Model(&server).
			Update("locked", true).Error; err != nil {
			return err
		}
		if err := tx.Model(&server).
			Update("ssh_key_assigned", req.GetPubkey()).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     false,
			AddressedIp: "",
		}, err
	}
	log.Printf("Server %s attribué à l'utilisateur %s\n", server.IP_Address, req.GetUserId())
	if err := s.installSSHKey(&server, req.GetPubkey()); err != nil {
		_ = s.DB.Model(&server).Update("locked", false)
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     false,
			AddressedIp: "",
		}, status.Errorf(codes.Internal, "ssh setup failed: %v", err)
	}
	return &frontcontrolpb.AttribVMinPoolResponse{
		Success:     true,
		AddressedIp: server.IP_Address,
	}, nil
}

func (s *Service) installSSHKey(server *models.Server, pubKey string) error {

	if err := godotenv.Load(); err != nil {
		log.Panicln("Error on loading .env")
		return fmt.Errorf("Error on loading.env")
	}
	signer, err := loadPrivateKey(os.Getenv("SSH_PRIVATE_KEY_PATH"))
	if err != nil {
		log.Printf("Erreur loadPrivateKey")
		return fmt.Errorf("load private key: %w", err)
	}

	config := sshConfig("vmuser", signer)
	addr := fmt.Sprintf("%s:22", server.IP_Address)

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		log.Printf("Erreur ssh dial")
		return fmt.Errorf("ssh dial failed: %w", err)
	}
	defer client.Close()

	var user models.User
	if err := s.DB.
		Where("email = ?", server.UserID).
		First(&user).Error; err != nil {
		log.Printf("Erreur fetch user from db")
		return fmt.Errorf("fetch user from db failed: %w", err)
	}

	appUsername := usernameFromEmail(user.Email)

	cmd := fmt.Sprintf(`
set -e

create_user_and_key() {
  USERNAME="$1"
  PUBKEY="$2"

  if ! id "$USERNAME" >/dev/null 2>&1; then
    sudo /usr/sbin/useradd -m -s /bin/bash "$USERNAME"
  fi

  HOME_DIR="/home/$USERNAME"
  SSH_DIR="$HOME_DIR/.ssh"
  AUTH_KEYS="$SSH_DIR/authorized_keys"

  sudo mkdir -p "$SSH_DIR"
  sudo chmod 700 "$SSH_DIR"
  sudo touch "$AUTH_KEYS"
  sudo chmod 600 "$AUTH_KEYS"

  if ! sudo grep -qxF "$PUBKEY" "$AUTH_KEYS"; then
    echo "$PUBKEY" | sudo tee -a "$AUTH_KEYS" > /dev/null
  fi

  sudo chown -R "$USERNAME:$USERNAME" "$SSH_DIR"
}

create_user_and_key "student" "%s"
create_user_and_key "%s" "%s"
`, pubKey, appUsername, user.Keypubuser)

	if err := runSSHcmd(client, cmd); err != nil {
		log.Printf("Erreur run ssh cmd")
		return fmt.Errorf("run ssh cmd failed: %w", err)
	}

	return nil
}

func loadPrivateKey(path string) (ssh.Signer, error) {
	log.Printf("path: %s\n", path)
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(key)
}

func sshConfig(user string, signer ssh.Signer) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
}

func runSSHcmd(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var stderr bytes.Buffer
	var stdout bytes.Buffer

	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		log.Printf("SSH stdout: %s", stdout.String())
		log.Printf("SSH stderr: %s", stderr.String())
		if stderr.Len() > 0 {
			return fmt.Errorf("ssh command error: %s", stderr.String())
		}
		return fmt.Errorf("ssh command failed: %w", err)
	}

	return nil
}

func usernameFromEmail(email string) string {
	local := strings.Split(email, "@")[0]
	local = strings.ToLower(local)

	// remplacer caractères interdits
	re := regexp.MustCompile(`[^a-z0-9_-]`)
	local = re.ReplaceAllString(local, "")

	if len(local) > 32 {
		local = local[:32]
	}

	return local
}
