package attribvm

import (
	"context"
	"control_center/frontcontrolpb"
	"control_center/internal/sshinject"
	"control_center/models"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey)); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	type result struct {
		ServerpoolID string
		UserID       string
	}

	var results []result

	err := s.DB.
		Table("students").
		Select("serverpools.serverpool_id, serverpools.user_id").
		Joins("JOIN list_students ON list_students.id = students.list_id").
		Joins("JOIN serverpools ON serverpools.id = list_students.pool_id").
		Where("students.ssh_key = ?", pubKey).
		Scan(&results).Error

	if err != nil {
		return status.Errorf(codes.Internal, "database error: %v", err)
	}

	for _, r := range results {
		resp := &frontcontrolpb.PoolWithKeyResponse{
			PoolId: r.ServerpoolID,
			UserId: r.UserID,
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
	if req.GetServerpoolId() == "" || req.GetPubkey() == "" || req.GetUserId() == "" {
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     false,
			AddressedIp: "",
		}, status.Error(codes.InvalidArgument, "missing required fields")
	}

	var student models.Student
	err := s.DB.
		Joins("JOIN list_students ON list_students.id = students.list_id").
		Joins("JOIN serverpools ON serverpools.id = list_students.pool_id").
		Where("students.ssh_key = ? AND serverpools.serverpool_id = ? AND serverpools.user_id = ?", req.GetPubkey(), req.GetServerpoolId(), req.GetUserId()).
		First(&student).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &frontcontrolpb.AttribVMinPoolResponse{
				Success:     false,
				AddressedIp: "",
			}, status.Error(codes.NotFound, "student not found in pool")
		}
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     false,
			AddressedIp: "",
		}, err
	}

	if student.IP != "" {
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     true,
			AddressedIp: student.IP,
		}, nil
	}

	var server models.Server

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("serverpool_id = ? AND user_id = ? AND locked = false AND name <> ?",
				req.GetServerpoolId(), req.GetUserId(),
				fmt.Sprintf("%s-%s-NFS", req.GetUserId(), req.GetServerpoolId())).
			Order("id").First(&server).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.ResourceExhausted, "no available server")
			}
			return err
		}
		if err := tx.Model(&server).
			Updates(map[string]any{
				"locked":           true,
				"ssh_key_assigned": student.SshKey,
			}).Error; err != nil {
			return err
		}

		if err := tx.Model(&student).Update("ip", server.IP_Address).Error; err != nil {
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

	if err := s.installSSHKey(&server, &student); err != nil {
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

func (s *Service) installSSHKey(server *models.Server, student *models.Student) error {
	signer, err := sshinject.LoadPrivateKey(os.Getenv("SSH_PRIVATE_KEY_PATH"))
	if err != nil {
		return err
	}

	config := sshinject.SshConfig("vmuser", signer)
	addr := fmt.Sprintf("%s:22", server.IP_Address)

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	defer client.Close()

	var user models.User
	if err := s.DB.
		Where("email = ?", server.UserID).
		First(&user).Error; err != nil {
		return fmt.Errorf("fetch user failed: %w", err)
	}
	//mettre un retry ici
	cmd := cmdInit(*student)
	log.Println("cmdInit")
	if err := sshinject.RunSSHcmd(client, cmd); err != nil {
		return fmt.Errorf("run ssh cmd failed: %w", err)
	}
	log.Println("ensureLocalDepotFolder")
	if err := ensureLocalDepotFolder(*student); err != nil {
		return err
	}

	// VM distante
	log.Println("ensureRemoteMountPoint")
	if err := sshinject.RunSSHcmd(client, ensureRemoteMountPointCmd()); err != nil {
		return err
	}

	log.Println("rCloneConfig")
	if err := sshinject.RunSSHcmd(client, rCloneConfigCmd("157.136.252.74")); err != nil {
		return err
	}

	log.Println("rcloneMount")
	if err := sshinject.RunSSHcmd(client, rcloneMountCmd(*student)); err != nil {
		return err
	}

	log.Println("rCloneConfig")
	if err := sshinject.RunSSHcmd(client, rCloneConfig(*student)); err != nil {
		return fmt.Errorf("run rclone config failed: %w", err)
	}
	return nil
}

func cmdInit(student models.Student) string {
	studentUsername := sshinject.UsernameFromEmail(student.Name)

	cmd := fmt.Sprintf(`
set -e

POOL_MOUNT="/mnt/pool"
POOL_GROUP="pool_prof"

ensure_group() {
  if ! getent group "$POOL_GROUP" >/dev/null; then
    sudo groupadd "$POOL_GROUP"
  fi
}

create_user() {
  USERNAME="$1"
  PUBKEY="$2"
  ROLE="$3" # student | prof

  if ! id "$USERNAME" >/dev/null 2>&1; then
    sudo useradd -m -s /bin/bash "$USERNAME"
  fi

  HOME="/home/$USERNAME"
  SSH="$HOME/.ssh"
  AUTH="$SSH/authorized_keys"

  sudo mkdir -p "$SSH"
  sudo chmod 700 "$SSH"
  sudo touch "$AUTH"
  sudo chmod 600 "$AUTH"

  if ! sudo grep -qxF "$PUBKEY" "$AUTH"; then
    echo "$PUBKEY" | sudo tee -a "$AUTH" > /dev/null
  fi

  # Prof → sudo + écriture
  if [ "$ROLE" = "prof" ]; then
    sudo usermod -aG sudo "$USERNAME"
    sudo usermod -aG "$POOL_GROUP" "$USERNAME"
  fi

  sudo chown -R "$USERNAME:$USERNAME" "$SSH"

  # Lien vers le pool
  if [ ! -L "$HOME/pool" ]; then
    sudo ln -s "$POOL_MOUNT" "$HOME/pool"
  fi

  sudo chown -h "$USERNAME:$USERNAME" "$HOME/pool"
}

ensure_group

# étudiant (lecture seule)
create_user "%s" "%s" "student"
`,
		studentUsername,
		student.SshKey,
	)

	return cmd
}

func rCloneConfig(student models.Student) string {
	mountPoint := "/home/" + sshinject.UsernameFromEmail(student.Name) + "/pool"
	remotePath := "depot:" + sshinject.UsernameFromEmail(student.Name)

	return fmt.Sprintf(`
	set -e
	
	mkdir -p%[1]s
	sudo chown %[2]s:%[2]s %[1]s
	
	if moutnpoint -q %[1]s; then
		echo "Already mounted"
		exit 0
	fi
	
	nohup rclone mount %[3]s %[1]s \
    --vfs-cache-mode writes \
    --buffer-size 64M \
    --dir-cache-time 1h \
    --allow-other \
    --log-file /home/%[2]s/.rclone_mount.log \
    --log-level INFO \
    > /dev/null 2>&1 &

	
	sleep 2
	
	mountpoint -1 %[1]s || exit 1
	`, mountPoint, sshinject.UsernameFromEmail(student.Name), remotePath)
}

func ensureLocalDepotFolder(student models.Student) error {
	studentUsername := sshinject.UsernameFromEmail(student.Name)
	depotPath := filepath.Join("/home/ubuntu/depot", studentUsername)

	if err := os.MkdirAll(depotPath, 0700); err != nil {
		log.Printf("local depot mkdir failed: %w", err)
		return fmt.Errorf("failed to create depot folder: %w", err)
	}

	return nil
}

func ensureRemoteMountPointCmd() string {
	return `
		mkdir -p /home/vmuser/depot &&
		chmod 700 /home/vmuser/depot
	`
}

func rcloneMountCmd(student models.Student) string {
	username := sshinject.UsernameFromEmail(student.Name)

	return fmt.Sprintf(`
		rclone mount depot:%[1]s /home/%[1]s/depot \
			--daemon \
			--vfs-cache-mode writes
	`, username)
}

func rCloneConfigCmd(serverIP string) string {
	return fmt.Sprintf(`
		mkdir -p /home/vmuser/.config/rclone

		cat > /home/vmuser/.config/rclone/rclone.conf << 'EOF'
[depot]
type = sftp
host = %[1]s
user = ubuntu
key_file = /home/vmuser/.ssh/id_ed25519
shell_type = unix
md5sum_command = none
sha1sum_command = none
EOF

		chmod 600 /home/vmuser/.config/rclone/rclone.conf
	`, serverIP)
}

func ensureRemoteSSHKeyCmd() string {
	return `
		mkdir -p ~/.ssh
		if [ ! -f ~/.ssh/id_ed25519 ]; then
			ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N ""
		fi
		chmod 700 ~/.ssh
		chmod 600 ~/.ssh/id_ed25519
	`
}

func authorizeDepotKey(pubKey string) error {
	authKeys := "/home/ubuntu/.ssh/authorized_keys"

	f, err := os.OpenFile(authKeys, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(pubKey + "\n")
	return err
}
