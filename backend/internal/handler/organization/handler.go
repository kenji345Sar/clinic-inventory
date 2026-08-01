package organization

import (
	"net/http"

	orgapp "clinic-inventory/internal/application/organization"
	orgdomain "clinic-inventory/internal/domain/organization"
	"clinic-inventory/internal/handler/httputil"
)

// Handler は組織コンテキストのHTTP受け口。
// 書き込みはユースケース経由、一覧などの単純な読み取りはリポジトリを直接使う。
type Handler struct {
	createCorporation *orgapp.CreateCorporationUseCase
	createFacility    *orgapp.CreateFacilityUseCase
	facilityRepo      orgdomain.FacilityRepository
}

func New(
	createCorporation *orgapp.CreateCorporationUseCase,
	createFacility *orgapp.CreateFacilityUseCase,
	facilityRepo orgdomain.FacilityRepository,
) *Handler {
	return &Handler{
		createCorporation: createCorporation,
		createFacility:    createFacility,
		facilityRepo:      facilityRepo,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/corporations", h.postCorporation)
	mux.HandleFunc("POST /api/facilities", h.postFacility)
	mux.HandleFunc("GET /api/facilities", h.listFacilities)
}

type corporationResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (h *Handler) postCorporation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, err)
		return
	}
	corporation, err := h.createCorporation.Execute(r.Context(), req.Name)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, corporationResponse{
		ID:   corporation.ID().String(),
		Name: corporation.Name(),
	})
}

type facilityResponse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	FacilityType  string  `json:"facilityType"`
	CorporationID string  `json:"corporationId"`
	GroupID       *string `json:"groupId"`
}

func toFacilityResponse(f *orgdomain.Facility) facilityResponse {
	var groupID *string
	if f.GroupID() != nil {
		s := f.GroupID().String()
		groupID = &s
	}
	return facilityResponse{
		ID:            f.ID().String(),
		Name:          f.Name(),
		FacilityType:  string(f.Type()),
		CorporationID: f.CorporationID().String(),
		GroupID:       groupID,
	}
}

func (h *Handler) postFacility(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		FacilityType  string `json:"facilityType"`
		CorporationID string `json:"corporationId"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, err)
		return
	}
	corporationID, err := httputil.ParseID(req.CorporationID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	facility, err := h.createFacility.Execute(r.Context(), orgapp.CreateFacilityInput{
		Name:          req.Name,
		FacilityType:  orgdomain.FacilityType(req.FacilityType),
		CorporationID: corporationID,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, toFacilityResponse(facility))
}

// listFacilities はクリニック一覧を返す。facility_userロールのユーザーには
// 自分の所属クリニックだけを返す(一覧そのものが認可の境界になる)。
// adminロール、またはロール未設定(認可の段階的導入中)のユーザーには全件返す。
func (h *Handler) listFacilities(w http.ResponseWriter, r *http.Request) {
	facilities, err := h.facilityRepo.FindAll(r.Context())
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	if claims, ok := httputil.AppClaimsFrom(r.Context()); ok && claims.Role == httputil.RoleFacilityUser {
		filtered := make([]*orgdomain.Facility, 0, 1)
		for _, f := range facilities {
			if f.ID().String() == claims.FacilityID {
				filtered = append(filtered, f)
			}
		}
		facilities = filtered
	}
	res := make([]facilityResponse, 0, len(facilities))
	for _, f := range facilities {
		res = append(res, toFacilityResponse(f))
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}
