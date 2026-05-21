package attribvm

import (
	"context"
	"control_center/frontcontrolpb"
	"control_center/internal/rclone"
	"control_center/internal/sshinject"
	"control_center/models"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

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

	// Return pools where there are unlocked servers OR this student already has a VM assigned
	err := s.DB.Raw(`
		SELECT DISTINCT sp.serverpool_id, sp.user_id
		FROM serverpools sp
		WHERE
			EXISTS (
				SELECT 1 FROM servers s
				WHERE s.serverpool_id = sp.serverpool_id
				AND s.user_id = sp.user_id
				AND s.locked = false
			)
			OR EXISTS (
				SELECT 1 FROM list_students ls
				JOIN students st ON st.list_id = ls.id
				WHERE ls.pool_id = sp.id
				AND (split_part(st.ssh_key, ' ', 1) || ' ' || split_part(st.ssh_key, ' ', 2) =
				     split_part(?, ' ', 1) || ' ' || split_part(?, ' ', 2))
				AND st.ip IS NOT NULL AND st.ip != ''
			)
	`, pubKey, pubKey).Scan(&results).Error

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
		Where("split_part(students.ssh_key, ' ', 1) || ' ' || split_part(students.ssh_key, ' ', 2) = split_part(?, ' ', 1) || ' ' || split_part(?, ' ', 2) AND serverpools.serverpool_id = ? AND serverpools.user_id = ?", req.GetPubkey(), req.GetPubkey(), req.GetServerpoolId(), req.GetUserId()).
		First(&student).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Auto-register student if not found
			var pool models.Serverpool
			if err := s.DB.Where("serverpool_id = ? AND user_id = ?", req.GetServerpoolId(), req.GetUserId()).First(&pool).Error; err != nil {
				return &frontcontrolpb.AttribVMinPoolResponse{
					Success: false,
					AddressedIp: "",
				}, status.Errorf(codes.NotFound, "pool not found")
			}
			
			var list models.ListStudents
			if err := s.DB.Where("pool_id = ?", pool.ID).FirstOrCreate(&list, models.ListStudents{PoolId: pool.ID}).Error; err != nil {
				return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, err
			}

			student = models.Student{
				ListId: list.ID,
				Name:   "student_" + req.GetServerpoolId(),
				SshKey: req.GetPubkey(),
			}
			if err := s.DB.Create(&student).Error; err != nil {
				return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, err
			}
		} else {
			return &frontcontrolpb.AttribVMinPoolResponse{
				Success:     false,
				AddressedIp: "",
			}, err
		}
	}

	if student.IP != "" {
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     true,
			AddressedIp: student.IP,
			Username:    sshinject.UsernameFromEmail(student.Name),
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
		_ = s.DB.Model(&student).Update("ip", "") // Clear the IP so they can try again
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     false,
			AddressedIp: "",
		}, status.Errorf(codes.Internal, "ssh setup failed: %v", err)
	}

	if os.Getenv("SKIP_RCLONE") != "true" {
		if err := rclone.SetupRcloneForStudent(server, student, req.GetUserId(), req.GetServerpoolId()); err != nil {
			_ = s.DB.Model(&server).Update("locked", false)
			_ = s.DB.Model(&student).Update("ip", "")
			return &frontcontrolpb.AttribVMinPoolResponse{
				Success:     false,
				AddressedIp: "",
			}, status.Errorf(codes.Internal, "rclone setup failed: %v", err)
		}
	} else {
		log.Println("[attribvm] SKIP_RCLONE=true, skipping rclone setup")
	}

	return &frontcontrolpb.AttribVMinPoolResponse{
		Success:     true,
		AddressedIp: server.IP_Address,
		Username:    sshinject.UsernameFromEmail(student.Name),
	}, nil
}

func (s *Service) installSSHKey(server *models.Server, student *models.Student) error {
	signer, err := sshinject.LoadPrivateKey(os.Getenv("SSH_PRIVATE_KEY_PATH"))
	if err != nil {
		return err
	}

	config := sshinject.SshConfig("vmuser", signer)
	addr := fmt.Sprintf("%s:22", server.IP_Address)

	var client *ssh.Client
	var dialErr error
	for i := 0; i < 12; i++ {
		client, dialErr = ssh.Dial("tcp", addr, config)
		if dialErr == nil {
			break
		}
		log.Printf("ssh.Dial failed, retrying in 5 seconds... (%v)", dialErr)
		time.Sleep(5 * time.Second)
	}
	if dialErr != nil {
		return fmt.Errorf("failed to connect to VM after retries: %v", dialErr)
	}
	defer client.Close()

	cmd := cmdInit(*student)
	log.Println("cmdInit")
	if err := sshinject.RunSSHcmd(client, cmd); err != nil {
		return fmt.Errorf("run ssh cmd failed: %w", err)
	}

	return nil
}

func cmdInit(student models.Student) string {
	studentUsername := sshinject.UsernameFromEmail(student.Name)

	cmd := fmt.Sprintf(`
set -e
USERNAME="%s"
PUBKEY="%s"
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

sudo chown -R "$USERNAME:$USERNAME" "$SSH"
`, studentUsername, student.SshKey)
	return cmd
}
