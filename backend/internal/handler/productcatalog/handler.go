package productcatalog

import (
	"net/http"

	prodapp "clinic-inventory/internal/application/productcatalog"
	proddomain "clinic-inventory/internal/domain/productcatalog"
	"clinic-inventory/internal/handler/httputil"
)

// Handler は商品マスタコンテキストのHTTP受け口。
type Handler struct {
	registerProduct   *prodapp.RegisterClinicProductUseCase
	clinicProductRepo proddomain.ClinicProductRepository
}

func New(
	registerProduct *prodapp.RegisterClinicProductUseCase,
	clinicProductRepo proddomain.ClinicProductRepository,
) *Handler {
	return &Handler{
		registerProduct:   registerProduct,
		clinicProductRepo: clinicProductRepo,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/facilities/{facilityId}/products", h.postProduct)
	mux.HandleFunc("GET /api/facilities/{facilityId}/products", h.listProducts)
}

type clinicProductResponse struct {
	ID                   string `json:"id"`
	FacilityID           string `json:"facilityId"`
	ProductCode          string `json:"productCode"`
	Name                 string `json:"name"`
	DistributorProductID string `json:"distributorProductId"`
	JANCode              string `json:"janCode"`
	ReorderPoint         int    `json:"reorderPoint"`
}

func toClinicProductResponse(p *proddomain.ClinicProduct) clinicProductResponse {
	return clinicProductResponse{
		ID:                   p.ID().String(),
		FacilityID:           p.FacilityID().String(),
		ProductCode:          p.ProductCode(),
		Name:                 p.Name(),
		DistributorProductID: p.DistributorProductID().String(),
		JANCode:              p.JANCode(),
		ReorderPoint:         p.ReorderPoint(),
	}
}

func (h *Handler) postProduct(w http.ResponseWriter, r *http.Request) {
	facilityID, err := httputil.ParseID(r.PathValue("facilityId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	var req struct {
		ProductCode          string `json:"productCode"`
		Name                 string `json:"name"`
		DistributorProductID string `json:"distributorProductId"`
		JANCode              string `json:"janCode"`
		ReorderPoint         int    `json:"reorderPoint"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, err)
		return
	}
	distributorProductID, err := httputil.ParseID(req.DistributorProductID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	product, err := h.registerProduct.Execute(r.Context(), prodapp.RegisterClinicProductInput{
		FacilityID:           facilityID,
		ProductCode:          req.ProductCode,
		Name:                 req.Name,
		DistributorProductID: distributorProductID,
		JANCode:              req.JANCode,
		ReorderPoint:         req.ReorderPoint,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, toClinicProductResponse(product))
}

// listProducts はクリニック商品の一覧を返す。
// クエリパラメータ jan が指定された場合はJANでの引き当て（バーコード消費の入口）として動作し、
// 該当1件のみの配列（見つからなければ404）を返す。
func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	facilityID, err := httputil.ParseID(r.PathValue("facilityId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	if jan := r.URL.Query().Get("jan"); jan != "" {
		product, err := h.clinicProductRepo.FindByFacilityAndJAN(r.Context(), facilityID, jan)
		if err != nil {
			httputil.WriteError(w, err)
			return
		}
		httputil.WriteJSON(w, http.StatusOK, []clinicProductResponse{toClinicProductResponse(product)})
		return
	}

	products, err := h.clinicProductRepo.FindByFacility(r.Context(), facilityID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	res := make([]clinicProductResponse, 0, len(products))
	for _, p := range products {
		res = append(res, toClinicProductResponse(p))
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}
