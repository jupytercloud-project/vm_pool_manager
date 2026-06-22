package grpc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"control_center/config"
	"control_center/internal/attribvm"
	"control_center/internal/moodle"
	"control_center/models"

	"github.com/danielgtaylor/huma/v2"
)

// writeJSONMoodle est un petit helper de réponse JSON (encore utilisé par les handlers
// bruts non migrés vers HUMA — vm-activity/guac/app-status, nbgrader, github).
func writeJSONMoodle(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// registerMoodleHuma enregistre tous les endpoints /api/moodle/*.
func registerMoodleHuma(api huma.API) {
	// GET /api/moodle/status — Moodle configuré ? (public)
	huma.Register(api, huma.Operation{
		OperationID: "moodle-status", Method: http.MethodGet, Path: "/api/moodle/status",
		Summary: "Disponibilité de l'intégration Moodle", Tags: []string{"moodle"},
	}, func(ctx context.Context, _ *struct{}) (*AnyOutput, error) {
		resp := map[string]any{"configured": moodle.Configured()}
		if c, err := moodle.New(); err == nil {
			resp["url"] = c.BaseHost()
		}
		return &AnyOutput{Body: resp}, nil
	})

	// GET /api/moodle/courses — liste les cours Moodle (sélecteur d'import).
	huma.Register(api, huma.Operation{
		OperationID: "moodle-courses", Method: http.MethodGet, Path: "/api/moodle/courses",
		Summary: "Lister les cours Moodle", Tags: []string{"moodle"},
	}, func(ctx context.Context, _ *struct{}) (*AnyOutput, error) {
		c, err := moodle.New()
		if err != nil {
			return nil, huma.Error503ServiceUnavailable(err.Error())
		}
		courses, err := c.GetCourses()
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		return &AnyOutput{Body: map[string]any{"courses": courses}}, nil
	})

	// GET /api/moodle/enrolments?course_id=X — élèves inscrits (aperçu avant import).
	huma.Register(api, huma.Operation{
		OperationID: "moodle-enrolments", Method: http.MethodGet, Path: "/api/moodle/enrolments",
		Summary: "Élèves inscrits à un cours Moodle", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct {
		CourseID int `query:"course_id"`
	}) (*AnyOutput, error) {
		if in.CourseID <= 0 {
			return nil, huma.Error400BadRequest("course_id invalide")
		}
		c, err := moodle.New()
		if err != nil {
			return nil, huma.Error503ServiceUnavailable(err.Error())
		}
		users, err := c.GetEnrolledUsers(in.CourseID)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		out := make([]moodleStudentDTO, 0, len(users))
		for _, u := range users {
			out = append(out, moodleStudentDTO{
				MoodleID: u.ID, Email: u.Email, FullName: u.FullName, IsTeacher: u.IsTeacher(),
			})
		}
		return &AnyOutput{Body: map[string]any{"students": out}}, nil
	})

	// POST /api/moodle/import — importe les élèves d'un cours Moodle dans un pool.
	huma.Register(api, huma.Operation{
		OperationID: "moodle-import", Method: http.MethodPost, Path: "/api/moodle/import",
		Summary: "Importer les élèves d'un cours Moodle", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct{ Body moodleImportRequest }) (*AnyOutput, error) {
		return handleMoodleImport(in.Body)
	})

	// POST /api/moodle/login {username, password} — valide les identifiants, crée une session. (public)
	huma.Register(api, huma.Operation{
		OperationID: "moodle-login", Method: http.MethodPost, Path: "/api/moodle/login",
		Summary: "Connexion Moodle (élève)", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
	}) (*AnyOutput, error) {
		if in.Body.Username == "" || in.Body.Password == "" {
			return nil, huma.Error400BadRequest("identifiant et mot de passe requis")
		}
		c, err := moodle.New()
		if err != nil {
			return nil, huma.Error503ServiceUnavailable(err.Error())
		}
		// 1) Valide les identifiants : un token renvoyé = identifiants corrects.
		if _, err := c.LoginToken(in.Body.Username, in.Body.Password, "moodle_mobile_app"); err != nil {
			return nil, huma.Error401Unauthorized("identifiants Moodle invalides")
		}
		// 2) Récupère l'identité via le token de service (le token utilisateur n'a pas accès à l'email).
		u, err := c.UserByUsername(in.Body.Username)
		if err != nil || u.Email == "" {
			return nil, huma.Error502BadGateway("profil Moodle introuvable")
		}
		role := "student" // Login Moodle = flux étudiant (enseignants/admins via OIDC).

		sessionID := randomState()
		config.Database.Create(&models.MoodleSession{
			ID: sessionID, Email: u.Email, FullName: u.FullName, MoodleUserID: u.ID, Role: role,
		})
		// Purge des sessions de plus de 24 h.
		config.Database.Where("created_at < ?", time.Now().Add(-24*time.Hour)).Delete(&models.MoodleSession{})

		return &AnyOutput{Body: map[string]any{
			"session_id": sessionID, "email": u.Email, "fullname": u.FullName, "role": role,
		}}, nil
	})

	// GET /api/moodle/session?id= — identité d'une session Moodle.
	huma.Register(api, huma.Operation{
		OperationID: "moodle-session", Method: http.MethodGet, Path: "/api/moodle/session",
		Summary: "Identité d'une session Moodle", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct {
		ID string `query:"id"`
	}) (*AnyOutput, error) {
		if in.ID == "" {
			return nil, huma.Error400BadRequest("id manquant")
		}
		var sess models.MoodleSession
		if err := config.Database.First(&sess, "id = ?", in.ID).Error; err != nil {
			return nil, huma.Error404NotFound("session introuvable")
		}
		return &AnyOutput{Body: map[string]any{
			"email": sess.Email, "fullname": sess.FullName, "role": sess.Role,
		}}, nil
	})

	// GET /api/moodle/my-pools?email= — pools où cet email (Moodle) est inscrit.
	huma.Register(api, huma.Operation{
		OperationID: "moodle-my-pools", Method: http.MethodGet, Path: "/api/moodle/my-pools",
		Summary: "Pools de l'élève", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct {
		Email string `query:"email"`
	}) (*AnyOutput, error) {
		// Anti-IDOR : l'email est dérivé de l'identité authentifiée ; seul un admin peut interroger un autre email.
		email := effectiveEmailCtx(ctx, in.Email)
		if email == "" {
			return nil, huma.Error400BadRequest("email manquant")
		}
		type poolRow struct {
			PoolID string `json:"pool_id"`
			UserID string `json:"user_id"`
		}
		var pools []poolRow
		config.Database.Raw(`
			SELECT DISTINCT sp.serverpool_id AS pool_id, sp.user_id AS user_id
			FROM serverpools sp
			JOIN list_students ls ON ls.pool_id = sp.id
			JOIN students st ON st.list_id = ls.id
			WHERE LOWER(st.moodle_email) = LOWER(?) AND sp.serverpool_id <> ''`, email).Scan(&pools)
		return &AnyOutput{Body: map[string]any{"pools": pools}}, nil
	})

	// POST /api/moodle/ssh-key {email, ssh_key} — ajoute/maj la clé SSH d'un élève.
	huma.Register(api, huma.Operation{
		OperationID: "moodle-ssh-key", Method: http.MethodPost, Path: "/api/moodle/ssh-key",
		Summary: "Définir la clé SSH d'un élève", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			Email  string `json:"email"`
			SSHKey string `json:"ssh_key"`
		}
	}) (*AnyOutput, error) {
		if in.Body.SSHKey == "" {
			return nil, huma.Error400BadRequest("ssh_key requis")
		}
		// Anti-IDOR : un élève ne peut poser une clé QUE sur son propre compte.
		email := effectiveEmailCtx(ctx, in.Body.Email)
		if email == "" {
			return nil, huma.Error400BadRequest("identité requise")
		}
		if err := attribvm.New(config.Database).SetStudentKeyByEmail(email, strings.TrimSpace(in.Body.SSHKey)); err != nil {
			return &AnyOutput{Body: map[string]any{"success": false, "error": err.Error()}}, nil
		}
		return &AnyOutput{Body: map[string]any{"success": true}}, nil
	})

	// POST /api/moodle/link-pool {pool_id, user_id, course_id} — lie un pool à un cours Moodle.
	huma.Register(api, huma.Operation{
		OperationID: "moodle-link-pool", Method: http.MethodPost, Path: "/api/moodle/link-pool",
		Summary: "Lier un pool à un cours Moodle", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			PoolID   string `json:"pool_id"`
			UserID   string `json:"user_id"`
			CourseID int    `json:"course_id"`
		}
	}) (*AnyOutput, error) {
		req := in.Body
		if req.PoolID == "" || req.UserID == "" || req.CourseID <= 0 {
			return nil, huma.Error400BadRequest("pool_id, user_id, course_id requis")
		}
		res := config.Database.Model(&models.Serverpool{}).
			Where("serverpool_id = ? AND user_id = ?", req.PoolID, req.UserID).
			Update("moodle_course_id", req.CourseID)
		if res.Error != nil || res.RowsAffected == 0 {
			return nil, huma.Error404NotFound("pool introuvable")
		}
		return &AnyOutput{Body: map[string]any{"success": true, "course_id": req.CourseID}}, nil
	})

	// GET /api/moodle/assignments?course_id=X (ou ?pool_id=&user_id=) — devoirs Moodle.
	huma.Register(api, huma.Operation{
		OperationID: "moodle-assignments", Method: http.MethodGet, Path: "/api/moodle/assignments",
		Summary: "Devoirs Moodle d'un cours", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct {
		CourseID int    `query:"course_id"`
		PoolID   string `query:"pool_id"`
		UserID   string `query:"user_id"`
	}) (*AnyOutput, error) {
		courseID := in.CourseID
		if courseID <= 0 && in.PoolID != "" && in.UserID != "" {
			var pool models.Serverpool
			if err := config.Database.Where("serverpool_id = ? AND user_id = ?", in.PoolID, in.UserID).First(&pool).Error; err == nil {
				courseID = pool.MoodleCourseID
			}
		}
		if courseID <= 0 {
			// Pas de cours Moodle lié à ce pool : pas une erreur, juste rien à proposer.
			return &AnyOutput{Body: map[string]any{"assignments": []any{}, "course_id": 0}}, nil
		}
		c, err := moodle.New()
		if err != nil {
			return nil, huma.Error503ServiceUnavailable(err.Error())
		}
		assigns, err := c.GetAssignments(courseID)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		return &AnyOutput{Body: map[string]any{"assignments": assigns, "course_id": courseID}}, nil
	})

	// POST /api/moodle/push-grades — remonte les notes nbgrader vers un devoir Moodle.
	huma.Register(api, huma.Operation{
		OperationID: "moodle-push-grades", Method: http.MethodPost, Path: "/api/moodle/push-grades",
		Summary: "Pousser les notes vers Moodle", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			PoolID         string `json:"pool_id"`
			UserID         string `json:"user_id"`
			Assignment     string `json:"assignment"`
			MoodleAssignID int    `json:"moodle_assign_id"`
		}
	}) (*AnyOutput, error) {
		return handleMoodlePushGrades(in.Body.PoolID, in.Body.UserID, in.Body.Assignment, in.Body.MoodleAssignID)
	})

	// POST /api/moodle/attrib-vm {pool_id, user_id, email} — attribue une VM sans clé SSH.
	huma.Register(api, huma.Operation{
		OperationID: "moodle-attrib-vm", Method: http.MethodPost, Path: "/api/moodle/attrib-vm",
		Summary: "Attribuer une VM (sans clé SSH)", Tags: []string{"moodle"},
	}, func(ctx context.Context, in *struct {
		Body struct {
			PoolID string `json:"pool_id"`
			UserID string `json:"user_id"`
			Email  string `json:"email"`
		}
	}) (*AnyOutput, error) {
		// Anti-IDOR : on attribue la VM à l'élève authentifié, jamais à un email arbitraire.
		email := effectiveEmailCtx(ctx, in.Body.Email)
		if email == "" {
			return nil, huma.Error400BadRequest("identité requise")
		}
		svc := attribvm.New(config.Database)
		ip, appPort, err := svc.AttribVMByEmail(in.Body.PoolID, in.Body.UserID, email)
		if err != nil {
			return &AnyOutput{Body: map[string]any{"success": false, "error": err.Error()}}, nil
		}
		return &AnyOutput{Body: map[string]any{"success": true, "ip": ip, "app_port": appPort}}, nil
	})
}

type moodleStudentDTO struct {
	MoodleID  int    `json:"moodle_id"`
	Email     string `json:"email"`
	FullName  string `json:"fullname"`
	IsTeacher bool   `json:"is_teacher"`
}

type moodleImportRequest struct {
	PoolID   string   `json:"pool_id"`
	UserID   string   `json:"user_id"`
	CourseID int      `json:"course_id"`
	Emails   []string `json:"emails"` // optionnel : restreint l'import à ces emails
}

// handleMoodleImport — importe les élèves d'un cours Moodle dans un pool.
// Crée une ligne students par élève (Name = email = id nbgrader, MoodleEmail, MoodleUserID),
// sans clé SSH (l'accès se fait via JupyterLab/Guacamole ; clé ajoutable plus tard).
func handleMoodleImport(req moodleImportRequest) (*AnyOutput, error) {
	if req.PoolID == "" || req.UserID == "" || req.CourseID <= 0 {
		return nil, huma.Error400BadRequest("pool_id, user_id et course_id requis")
	}

	c, err := moodle.New()
	if err != nil {
		return nil, huma.Error503ServiceUnavailable(err.Error())
	}
	users, err := c.GetEnrolledUsers(req.CourseID)
	if err != nil {
		return nil, huma.Error502BadGateway(err.Error())
	}

	// Filtre optionnel par emails sélectionnés.
	var only map[string]bool
	if len(req.Emails) > 0 {
		only = map[string]bool{}
		for _, e := range req.Emails {
			only[strings.ToLower(strings.TrimSpace(e))] = true
		}
	}

	// Pool + liste d'étudiants.
	var pool models.Serverpool
	if err := config.Database.Preload("ListStudents.Students").
		Where("serverpool_id = ? AND user_id = ?", req.PoolID, req.UserID).
		First(&pool).Error; err != nil {
		return nil, huma.Error404NotFound("pool introuvable")
	}
	list := &pool.ListStudents
	if list.ID == 0 {
		list.PoolId = pool.ID
		if err := config.Database.Create(list).Error; err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
	}
	existing := map[string]bool{}
	for _, s := range list.Students {
		if s.MoodleEmail != "" {
			existing[strings.ToLower(s.MoodleEmail)] = true
		}
	}

	imported, skipped := 0, 0
	for _, u := range users {
		if u.IsTeacher() || u.Email == "" {
			continue
		}
		key := strings.ToLower(u.Email)
		if only != nil && !only[key] {
			continue
		}
		if existing[key] {
			skipped++
			continue
		}
		student := models.Student{
			ListId:       list.ID,
			Name:         u.Email, // = identifiant nbgrader
			MoodleEmail:  u.Email,
			MoodleUserID: u.ID,
		}
		if err := config.Database.Create(&student).Error; err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		existing[key] = true
		imported++
	}

	// Mémorise le lien pool ↔ cours Moodle (pour le push de notes).
	config.Database.Model(&models.Serverpool{}).Where("id = ?", pool.ID).
		Update("moodle_course_id", req.CourseID)

	return &AnyOutput{Body: map[string]any{
		"imported": imported, "skipped": skipped, "course_id": req.CourseID,
	}}, nil
}

// handleMoodlePushGrades — remonte les notes nbgrader d'un assignment vers un devoir Moodle.
func handleMoodlePushGrades(poolID, userID, assignment string, moodleAssignID int) (*AnyOutput, error) {
	if poolID == "" || userID == "" || assignment == "" || moodleAssignID <= 0 {
		return nil, huma.Error400BadRequest("pool_id, user_id, assignment et moodle_assign_id requis")
	}
	c, err := moodle.New()
	if err != nil {
		return nil, huma.Error503ServiceUnavailable(err.Error())
	}

	// 1) Notes nbgrader (student = email).
	grades, err := fetchNbgraderGrades(poolID, userID, assignment)
	if err != nil {
		return nil, huma.Error502BadGateway("lecture des notes impossible: " + err.Error())
	}

	// 2) Map email → moodle_user_id (depuis les élèves importés du pool).
	var pool models.Serverpool
	if err := config.Database.Preload("ListStudents.Students").
		Where("serverpool_id = ? AND user_id = ?", poolID, userID).First(&pool).Error; err != nil {
		return nil, huma.Error404NotFound("pool introuvable")
	}
	uidByEmail := map[string]int{}
	for _, s := range pool.ListStudents.Students {
		if s.MoodleEmail != "" && s.MoodleUserID != 0 {
			uidByEmail[strings.ToLower(s.MoodleEmail)] = s.MoodleUserID
		}
	}

	// 3) Barème du devoir Moodle (pour mettre la note à l'échelle).
	maxGrade := 100.0
	if assigns, e := c.GetAssignments(pool.MoodleCourseID); e == nil {
		for _, a := range assigns {
			if a.ID == moodleAssignID {
				maxGrade = a.MaxGrade
			}
		}
	}

	pushed, skipped := 0, 0
	var failures []string
	for _, g := range grades {
		// Ne pas pousser de note aux étudiants qui n'ont rien rendu.
		if g.Status == "missing" {
			skipped++
			continue
		}
		uid := uidByEmail[strings.ToLower(g.Student)]
		if uid == 0 {
			skipped++
			continue
		}
		grade := g.Score
		if g.MaxScore > 0 {
			grade = g.Score / g.MaxScore * maxGrade
		}
		if err := c.SaveAssignGrade(moodleAssignID, uid, grade, ""); err != nil {
			failures = append(failures, g.Student)
			continue
		}
		pushed++
	}
	return &AnyOutput{Body: map[string]any{
		"pushed": pushed, "skipped": skipped, "failures": failures, "total": len(grades),
	}}, nil
}
