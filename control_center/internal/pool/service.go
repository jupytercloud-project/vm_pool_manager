package pool

import (
	"context"
	"log"
	"strconv"
	"time"

	"control_center/config"
	"control_center/frontcontrolpb"
	"control_center/models"
	"control_center/pb"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type Service struct {
	frontcontrolpb.UnimplementedPoolServiceServer
	DB *gorm.DB
	pm pb.PoolManagerClient
}

func New(db *gorm.DB, pm pb.PoolManagerClient) *Service {
	return &Service{
		DB: db,
		pm: pm,
	}
}

func (s *Service) CreatePool(
	ctx context.Context,
	req *frontcontrolpb.CreatePoolRequest,
) (*frontcontrolpb.CreatePoolResponse, error) {
	minVM, _ := strconv.Atoi(req.GetMinVm())
	maxVM, _ := strconv.Atoi(req.GetMaxVm())

	pool := models.Serverpool{
		UserID:       req.GetUser(),
		ServerpoolID: req.GetName(),
		ImageRef:     req.GetImage(),
		FlavorRef:    req.GetFlavor(),
		MinVM:        minVM,
		MaxVM:        maxVM,
		Networks:     models.JSONStringSlice{req.GetNetwork()},
		ConfigID:     req.GetConfig(),
		Status:       "scheduled",
	}
	pool.TimeStart = new(time.Time)
	*pool.TimeStart = req.GetStartTime().AsTime()

	// modif here to test timewindow and delete
	tw := time.Duration(req.GetTimeWindow()) * time.Hour
	pool.Timewindow = new(time.Duration)
	*pool.Timewindow = tw
	res := config.Database.Create(&pool)
	if res.Error != nil {
		return &frontcontrolpb.CreatePoolResponse{Success: false}, res.Error
	}
	return &frontcontrolpb.CreatePoolResponse{Success: true}, nil

}

func (s *Service) DeletePool(
	ctx context.Context,
	req *frontcontrolpb.DeletePoolRequest,
) (*frontcontrolpb.DeletePoolResponse, error) {
	var pool models.Serverpool
	if err := s.DB.Where(
		"serverpool_id = ? AND user_id = ?", req.GetPoolId(), req.GetUser(),
	).First(&pool).Error; err != nil {
		return &frontcontrolpb.DeletePoolResponse{Success: false}, err
	}

	if err := s.DB.Delete(&pool).Error; err != nil {
		return &frontcontrolpb.DeletePoolResponse{Success: false}, err
	}

	rep, err := s.pm.SendRessources(
		ctx,
		&pb.RessourceRequest{
			User:   req.GetUser(),
			Data:   pool.ToMap(),
			Status: pb.Status_DELETE,
			Type:   pb.Type_SERVERPOOL,
		},
	)

	if err != nil || rep.GetSuccess() == false {
		return &frontcontrolpb.DeletePoolResponse{Success: false}, err
	}
	log.Println("success deleting")
	return &frontcontrolpb.DeletePoolResponse{Success: true}, nil
}

func (s *Service) GetPool(
	ctx context.Context,
	req *frontcontrolpb.GetPoolRequest,
) (*frontcontrolpb.GetPoolResponse, error) {
	var pool models.Serverpool
	if err := s.DB.Where(
		"serverpool_id = ? AND user_id = ?", req.GetPoolId(), req.GetUser(),
	).First(&pool).Error; err != nil {
		return &frontcontrolpb.GetPoolResponse{}, err
	}

	return &frontcontrolpb.GetPoolResponse{
		Name:    pool.ServerpoolID,
		Image:   pool.ImageRef,
		Flavor:  pool.FlavorRef,
		MinVm:   int32(pool.MinVM),
		MaxVm:   int32(pool.MaxVM),
		Network: pool.Networks[0],
		Config:  pool.ConfigID,
	}, nil
}

func (s *Service) RebuildServer(
	ctx context.Context,
	req *frontcontrolpb.RebuildServerRequest,
) (*frontcontrolpb.RebuildServerResponse, error) {
	var server models.Server
	if err := s.DB.Where(
		"name = ? AND user_id = ?", req.GetServerId(), req.GetUser(),
	).First(&server).Error; err != nil {
		return &frontcontrolpb.RebuildServerResponse{Success: false}, err
	}

	data := server.ToMap()
	data["serverpool_id"] = req.GetPoolId()

	rep, err := s.pm.SendRessources(
		ctx,
		&pb.RessourceRequest{
			User:   req.GetUser(),
			Data:   data,
			Status: pb.Status_UPDATE,
			Type:   pb.Type_SERVER,
		},
	)

	if err != nil || !rep.GetSuccess() {
		return &frontcontrolpb.RebuildServerResponse{Success: false}, err
	}

	return &frontcontrolpb.RebuildServerResponse{Success: true}, nil
}

func (s *Service) AddServer(
	ctx context.Context,
	req *frontcontrolpb.CreatePoolRequest,
) (*frontcontrolpb.RebuildServerResponse, error) {
	var serv models.Server
	var pool models.Serverpool
	if err := s.DB.Where(
		"serverpool_id = ? AND user_id = ?", req.GetName(), req.GetUser(),
	).First(&pool).Error; err != nil {
		return &frontcontrolpb.RebuildServerResponse{Success: false}, err
	}
	serv = models.Server{
		UserID:       req.GetUser(),
		ImageRef:     pool.ImageRef,
		FlavorRef:    pool.FlavorRef,
		Networks:     pool.Networks,
		ServerpoolID: pool.ServerpoolID,
	}

	data := serv.ToMap()
	data["serverpool_id"] = pool.ServerpoolID
	data["config_id"] = pool.ConfigID

	rep, err := s.pm.SendRessources(
		ctx,
		&pb.RessourceRequest{
			User:   req.GetUser(),
			Data:   data,
			Status: pb.Status_CREATE,
			Type:   pb.Type_SERVER,
		},
	)

	if err != nil || !rep.GetSuccess() {
		return &frontcontrolpb.RebuildServerResponse{Success: false}, err
	}

	return &frontcontrolpb.RebuildServerResponse{Success: true}, nil
}

func (s *Service) AddSSHKeys(
	ctx context.Context,
	req *frontcontrolpb.ListSSHPublicKeysRequest,
) (*frontcontrolpb.ListSSHPublicKeysResponse, error) {
	var pool models.Serverpool
	if err := s.DB.Model(models.Serverpool{}).
		Where("serverpool_id = ? AND user_id = ?", req.GetServerpoolId(), req.GetUserId()).
		First(&pool).Error; err != nil {
		return &frontcontrolpb.ListSSHPublicKeysResponse{Success: false}, err
	}
	log.Printf("req keys: %v", req.GetPubkeys())
	for _, key := range req.GetPubkeys() {
		_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
		if err != nil {
			return &frontcontrolpb.ListSSHPublicKeysResponse{Success: false}, err
		}
		pool.Keypublist = append(pool.Keypublist, key)
	}
	if err := s.DB.Save(&pool).Error; err != nil {
		return &frontcontrolpb.ListSSHPublicKeysResponse{Success: false}, err
	}
	return &frontcontrolpb.ListSSHPublicKeysResponse{Success: true}, nil
}

func (s *Service) ListStudents(
	ctx context.Context,
	req *frontcontrolpb.ListStudentsRequest,
) (*frontcontrolpb.ListStudentsResponse, error) {
	var pool models.Serverpool
	if err := s.DB.Preload("ListStudents.Students").
		Where("serverpool_id = ? AND user_id = ?", req.GetPoolname(), req.GetUser()).
		First(&pool).Error; err != nil {
		return &frontcontrolpb.ListStudentsResponse{}, err
	}
	log.Println("test test")

	var students []*frontcontrolpb.Student
	for _, student := range pool.ListStudents.Students {
		students = append(students, &frontcontrolpb.Student{
			Name:   student.Name,
			SshKey: student.SshKey,
			Ip:     student.IP,
		})
	}

	return &frontcontrolpb.ListStudentsResponse{
		Students: students,
	}, nil
}

func (s *Service) AddStudents(
	ctx context.Context,
	req *frontcontrolpb.AddStudentRequest,
) (*frontcontrolpb.AddStudentResponse, error) {
	var pool models.Serverpool
	if err := s.DB.Preload("ListStudents.Students").
		Where("serverpool_id = ? AND user_id = ?", req.GetPoolname(), req.GetUser()).
		First(&pool).Error; err != nil {
		return &frontcontrolpb.AddStudentResponse{Success: false}, err
	}

	listStudents := &pool.ListStudents
	if listStudents.ID == 0 {
		listStudents.PoolId = pool.ID
		if err := s.DB.Create(&listStudents).Error; err != nil {
			return &frontcontrolpb.AddStudentResponse{Success: false}, err
		}
	}

	for _, studentReq := range req.GetStudents() {
		student := models.Student{
			ListId: listStudents.ID,
			Name:   studentReq.GetName(),
			SshKey: studentReq.GetSshKey(),
		}
		if err := s.DB.Create(&student).Error; err != nil {
			return &frontcontrolpb.AddStudentResponse{Success: false}, err
		}
	}

	return &frontcontrolpb.AddStudentResponse{Success: true}, nil
}
