package grpc

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

// newHumaAPI monte une API HUMA sur le mux REST existant. Les chemins /api/* et le
// middleware d'auth en amont (httpAuthMiddleware) sont inchangés ; les endpoints non
// encore migrés restent en mux.HandleFunc et coexistent (migration incrémentale).
// Fournit en plus l'OpenAPI 3.1 (/api/openapi.json|yaml) et la doc (/api/docs).
func newHumaAPI(mux *http.ServeMux) huma.API {
	// Format d'erreur identique à l'historique : {"error": "..."} (le frontend lit .error).
	huma.NewError = func(status int, msg string, _ ...error) huma.StatusError {
		return &apiError{status: status, Err: msg}
	}
	config := huma.DefaultConfig("CloudPoolManager API", "1.0.0")
	config.OpenAPIPath = "/api/openapi"
	config.DocsPath = "/api/docs"
	// Champs de body optionnels par défaut : on reproduit le json.Decode permissif
	// d'origine (la validation « requis » reste faite à la main dans chaque handler,
	// avec les messages 400 historiques). Sinon HUMA rendrait tout champ obligatoire.
	config.FieldsOptionalByDefault = true
	api := humago.New(mux, config)
	registerHumaRoutes(api)
	return api
}

// apiError : enveloppe d'erreur HUMA produisant {"error": "..."} (compat frontend).
type apiError struct {
	status int
	Err    string `json:"error"`
}

func (e *apiError) Error() string  { return e.Err }
func (e *apiError) GetStatus() int { return e.status }

// registerHumaRoutes enregistre les opérations migrées vers HUMA.
// On y déplace les endpoints au fur et à mesure (et on retire le mux.HandleFunc correspondant).
func registerHumaRoutes(api huma.API) {
	registerJobsHuma(api)
	registerUsageHuma(api)
	registerStorageHuma(api)
	registerXCoursHuma(api)
	registerAnnouncementHuma(api)
	registerAuditHuma(api)
	registerAdminUsersHuma(api)
	registerAdminConsoleHuma(api)
	registerVMHuma(api)
	registerPoolMetaHuma(api)
	registerPoolBroadcastHuma(api)
	registerPoolProgressHuma(api)
	registerPoolPresetsHuma(api)
	registerMoodleHuma(api)
	registerNbgraderHuma(api)
	registerGitHubHuma(api)
	registerImageProposalsHuma(api)

	// GET /api/me — identité + rôle effectif de l'appelant.
	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/api/me",
		Summary:     "Identité et rôle de l'appelant",
		Tags:        []string{"auth"},
	}, func(ctx context.Context, _ *struct{}) (*MeOutput, error) {
		id, ok := identityFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("non authentifié")
		}
		out := &MeOutput{}
		out.Body.Email = id.Email
		out.Body.Role = id.Role
		out.Body.IsAdmin = id.Role == RoleAdmin
		out.Body.IsStaff = isStaff(id.Role)
		out.Body.Via = id.Via
		return out, nil
	})

	// GET /api/inventory — inventaire des VMs groupées par pool.
	huma.Register(api, huma.Operation{
		OperationID: "get-inventory",
		Method:      http.MethodGet,
		Path:        "/api/inventory",
		Summary:     "Inventaire des VMs par pool",
		Tags:        []string{"inventory"},
	}, func(ctx context.Context, _ *struct{}) (*InventoryOutput, error) {
		pools, err := buildInventory()
		if err != nil {
			return nil, huma.Error500InternalServerError("inventaire indisponible", err)
		}
		return &InventoryOutput{Body: pools}, nil
	})

	// GET /api/pricing — tarifs unitaires (estimateur de coût).
	huma.Register(api, huma.Operation{
		OperationID: "get-pricing",
		Method:      http.MethodGet,
		Path:        "/api/pricing",
		Summary:     "Tarifs unitaires (vCPU·h, Go·h)",
		Tags:        []string{"usage"},
	}, func(ctx context.Context, _ *struct{}) (*PricingOutput, error) {
		out := &PricingOutput{}
		out.Body.Currency = priceCurrency()
		out.Body.VCPUHour = priceVCPUHour()
		out.Body.GBHour = priceGBHour()
		return out, nil
	})
}

// AnyOutput : réponse JSON dynamique (forme identique aux anciens handlers map[string]any).
// Le schéma OpenAPI reste générique (objet libre) mais le JSON émis est strictement le même.
type AnyOutput struct {
	Body any
}

// InventoryOutput : réponse de GET /api/inventory ([]InventoryPool, forme inchangée).
type InventoryOutput struct {
	Body []InventoryPool
}

// PricingOutput : réponse de GET /api/pricing.
type PricingOutput struct {
	Body struct {
		Currency string  `json:"currency"`
		VCPUHour float64 `json:"vcpu_hour"`
		GBHour   float64 `json:"gb_hour"`
	}
}

// MeOutput : réponse de GET /api/me (forme JSON identique à l'ancien handler).
type MeOutput struct {
	Body struct {
		Email   string `json:"email"`
		Role    string `json:"role"`
		IsAdmin bool   `json:"is_admin"`
		IsStaff bool   `json:"is_staff"`
		Via     string `json:"via"`
	}
}
