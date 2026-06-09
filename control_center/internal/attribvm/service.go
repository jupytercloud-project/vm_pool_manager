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

	// Return all pools where this student's SSH key is registered
	err := s.DB.Raw(`
		SELECT DISTINCT sp.serverpool_id, sp.user_id
		FROM serverpools sp
		WHERE
			sp.serverpool_id != '' AND sp.user_id != ''
			AND EXISTS (
				SELECT 1 FROM list_students ls
				JOIN students st ON st.list_id = ls.id
				WHERE ls.pool_id = sp.id
				AND (split_part(st.ssh_key, ' ', 1) || ' ' || split_part(st.ssh_key, ' ', 2) =
				     split_part(?, ' ', 1) || ' ' || split_part(?, ' ', 2))
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
	// NOTE: business-logic failures are returned as a normal response with
	// Success=false and a nil gRPC error. Returning a gRPC error here produces a
	// "trailers-only" response that the Caddy grpc_web module mis-encodes
	// (grpc-status:0 in the body trailer), which the frontend then surfaces as a
	// misleading "[unimplemented] missing message". Always delivering a message
	// keeps the real outcome readable; the precise reason is logged server-side.
	if req.GetServerpoolId() == "" || req.GetPubkey() == "" || req.GetUserId() == "" {
		log.Printf("[attribvm] missing required fields (serverpool_id/pubkey/user_id)")
		return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, nil
	}

	// Always load the pool to get app_port and other metadata.
	var pool models.Serverpool
	if err := s.DB.Where("serverpool_id = ? AND user_id = ?", req.GetServerpoolId(), req.GetUserId()).First(&pool).Error; err != nil {
		log.Printf("[attribvm] pool not found: %s/%s", req.GetServerpoolId(), req.GetUserId())
		return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, nil
	}

	var student models.Student
	err := s.DB.
		Joins("JOIN list_students ON list_students.id = students.list_id").
		Joins("JOIN serverpools ON serverpools.id = list_students.pool_id").
		Where("split_part(students.ssh_key, ' ', 1) || ' ' || split_part(students.ssh_key, ' ', 2) = split_part(?, ' ', 1) || ' ' || split_part(?, ' ', 2) AND serverpools.serverpool_id = ? AND serverpools.user_id = ?", req.GetPubkey(), req.GetPubkey(), req.GetServerpoolId(), req.GetUserId()).
		First(&student).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var list models.ListStudents
			if err := s.DB.Where("pool_id = ?", pool.ID).FirstOrCreate(&list, models.ListStudents{PoolId: pool.ID}).Error; err != nil {
				log.Printf("[attribvm] list lookup/create failed: %v", err)
				return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, nil
			}

			student = models.Student{
				ListId: list.ID,
				Name:   "student_" + req.GetServerpoolId(),
				SshKey: req.GetPubkey(),
			}
			if err := s.DB.Create(&student).Error; err != nil {
				log.Printf("[attribvm] student create failed: %v", err)
				return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, nil
			}
		} else {
			log.Printf("[attribvm] student lookup failed: %v", err)
			return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, nil
		}
	}

	if student.IP != "" {
		return &frontcontrolpb.AttribVMinPoolResponse{
			Success:     true,
			AddressedIp: student.IP,
			Username:    sshinject.UsernameFromEmail(student.Name),
			AppPort:     int32(pool.AppPort),
		}, nil
	}

	var server models.Server

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		// Identify the instructor VM (the oldest VM in the pool) to exclude it
		var instrServer models.Server
		tx.Where("serverpool_id = ? AND user_id = ?", req.GetServerpoolId(), req.GetUserId()).Order("created_at ASC").First(&instrServer)

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("serverpool_id = ? AND user_id = ? AND locked = false AND name <> ? AND id <> ?",
				req.GetServerpoolId(), req.GetUserId(),
				fmt.Sprintf("%s-%s-NFS", req.GetUserId(), req.GetServerpoolId()), instrServer.ID).
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
		log.Printf("[attribvm] attribution failed (pool %s/%s): %v", req.GetServerpoolId(), req.GetUserId(), err)
		return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, nil
	}

	if err := s.installSSHKey(&server, &student); err != nil {
		_ = s.DB.Model(&server).Update("locked", false)
		_ = s.DB.Model(&student).Update("ip", "") // Clear the IP so they can try again
		log.Printf("[attribvm] ssh setup failed on %s: %v", server.IP_Address, err)
		return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, nil
	}

	if os.Getenv("SKIP_RCLONE") != "true" {
		if err := rclone.SetupRcloneForStudent(server, student, req.GetUserId(), req.GetServerpoolId()); err != nil {
			_ = s.DB.Model(&server).Update("locked", false)
			_ = s.DB.Model(&student).Update("ip", "")
			log.Printf("[attribvm] rclone setup failed on %s: %v", server.IP_Address, err)
			return &frontcontrolpb.AttribVMinPoolResponse{Success: false}, nil
		}
	} else {
		log.Println("[attribvm] SKIP_RCLONE=true, skipping rclone setup")
	}

	return &frontcontrolpb.AttribVMinPoolResponse{
		Success:     true,
		AddressedIp: server.IP_Address,
		Username:    sshinject.UsernameFromEmail(student.Name),
		AppPort:     int32(pool.AppPort),
	}, nil
}

// AttribVMByEmail attribue une VM à un étudiant identifié par son email Moodle,
// SANS exiger de clé SSH : l'accès se fait via JupyterLab (navigateur) et le terminal
// Guacamole (clé gérée côté plateforme). L'étudiant doit avoir été importé dans le pool.
// Renvoie l'IP attribuée et le port applicatif.
func (s *Service) AttribVMByEmail(poolID, userID, email string) (string, int32, error) {
	if poolID == "" || userID == "" || email == "" {
		return "", 0, fmt.Errorf("pool_id, user_id et email requis")
	}

	var pool models.Serverpool
	if err := s.DB.Where("serverpool_id = ? AND user_id = ?", poolID, userID).First(&pool).Error; err != nil {
		return "", 0, fmt.Errorf("pool introuvable")
	}

	// Étudiant pré-importé depuis Moodle, identifié par MoodleEmail dans ce pool.
	var student models.Student
	err := s.DB.
		Joins("JOIN list_students ON list_students.id = students.list_id").
		Where("LOWER(students.moodle_email) = LOWER(?) AND list_students.pool_id = ?", email, pool.ID).
		First(&student).Error
	if err != nil {
		return "", 0, fmt.Errorf("vous n'êtes pas inscrit à ce cours")
	}

	// Déjà attribuée → on renvoie la même VM.
	if student.IP != "" {
		return student.IP, int32(pool.AppPort), nil
	}

	var server models.Server
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		// Exclure la VM instructeur (la plus ancienne) — cohérent avec AttribVMinPool.
		var instrServer models.Server
		tx.Where("serverpool_id = ? AND user_id = ?", poolID, userID).Order("created_at ASC").First(&instrServer)

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("serverpool_id = ? AND user_id = ? AND locked = false AND name <> ? AND id <> ?",
				poolID, userID,
				fmt.Sprintf("%s-%s-NFS", userID, poolID), instrServer.ID).
			Order("id").First(&server).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.ResourceExhausted, "no available server")
			}
			return err
		}
		if err := tx.Model(&server).Update("locked", true).Error; err != nil {
			return err
		}
		return tx.Model(&student).Update("ip", server.IP_Address).Error
	})
	if err != nil {
		log.Printf("[attribvm] attribution Moodle échouée (pool %s/%s, %s): %v", poolID, userID, email, err)
		if status.Code(err) == codes.ResourceExhausted {
			return "", 0, fmt.Errorf("aucune VM disponible")
		}
		return "", 0, fmt.Errorf("attribution impossible")
	}

	// Si l'étudiant a (plus tard) renseigné une clé SSH, on l'injecte ; sinon on s'arrête là
	// (accès navigateur/Guacamole, pas besoin de clé).
	if student.SshKey != "" {
		if err := s.installSSHKey(&server, &student); err != nil {
			log.Printf("[attribvm] injection clé (Moodle) échouée sur %s: %v", server.IP_Address, err)
		}
	}

	return server.IP_Address, int32(pool.AppPort), nil
}

// SetStudentKeyByEmail enregistre une clé SSH pour l'étudiant (identifié par email Moodle)
// et l'injecte dans sa VM si elle est déjà attribuée.
func (s *Service) SetStudentKeyByEmail(email, pubkey string) error {
	if email == "" || pubkey == "" {
		return fmt.Errorf("email et clé requis")
	}
	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubkey)); err != nil {
		return fmt.Errorf("clé SSH invalide")
	}
	var students []models.Student
	s.DB.Where("LOWER(moodle_email) = LOWER(?)", email).Find(&students)
	if len(students) == 0 {
		return fmt.Errorf("aucun étudiant pour cet email — rejoignez d'abord un cours")
	}
	for i := range students {
		s.DB.Model(&students[i]).Update("ssh_key", pubkey)
		students[i].SshKey = pubkey
		if students[i].IP != "" {
			var server models.Server
			if err := s.DB.Where("ip_address = ?", students[i].IP).First(&server).Error; err == nil {
				if err := s.installSSHKey(&server, &students[i]); err != nil {
					log.Printf("[attribvm] injection clé (ajout manuel) échouée sur %s: %v", server.IP_Address, err)
				}
			}
		}
	}
	return nil
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
