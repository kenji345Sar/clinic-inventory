package productcatalog

import (
	"context"
	"net/http"

	prodapp "clinic-inventory/internal/application/productcatalog"
	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	proddomain "clinic-inventory/internal/domain/productcatalog"
	"clinic-inventory/internal/handler/httputil"
)

// Handler は商品マスタコンテキストのHTTP受け口。
// 一覧表示では「その商品がどの卸のものか」を出すため、卸連携コンテキストの
// リポジトリを読み取り専用で参照し、卸商品→卸業者をたどって卸業者名を補完する。
type Handler struct {
	registerProduct        *prodapp.RegisterClinicProductUseCase
	clinicProductRepo      proddomain.ClinicProductRepository
	distributorProductRepo distdomain.DistributorProductRepository
	distributorRepo        distdomain.DistributorRepository
}

func New(
	registerProduct *prodapp.RegisterClinicProductUseCase,
	clinicProductRepo proddomain.ClinicProductRepository,
	distributorProductRepo distdomain.DistributorProductRepository,
	distributorRepo distdomain.DistributorRepository,
) *Handler {
	return &Handler{
		registerProduct:        registerProduct,
		clinicProductRepo:      clinicProductRepo,
		distributorProductRepo: distributorProductRepo,
		distributorRepo:        distributorRepo,
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
	DistributorID        string `json:"distributorId"`
	DistributorName      string `json:"distributorName"`
	JANCode              string `json:"janCode"`
	ReorderPoint         int    `json:"reorderPoint"`
}

// toClinicProductResponse はクリニック商品に卸業者名を補完してレスポンス化する。
// 卸商品→卸業者の解決に失敗しても表示自体は続けられるよう、卸情報は空のまま返す
// (一覧の一部が欠けても画面全体を落とさない)。
func (h *Handler) toClinicProductResponse(ctx context.Context, p *proddomain.ClinicProduct) clinicProductResponse {
	res := clinicProductResponse{
		ID:                   p.ID().String(),
		FacilityID:           p.FacilityID().String(),
		ProductCode:          p.ProductCode(),
		Name:                 p.Name(),
		DistributorProductID: p.DistributorProductID().String(),
		JANCode:              p.JANCode(),
		ReorderPoint:         p.ReorderPoint(),
	}
	distributorProduct, err := h.distributorProductRepo.FindByID(ctx, p.DistributorProductID())
	if err != nil {
		return res
	}
	res.DistributorID = distributorProduct.DistributorID().String()
	distributor, err := h.distributorRepo.FindByID(ctx, distributorProduct.DistributorID())
	if err != nil {
		return res
	}
	res.DistributorName = distributor.Name()
	return res
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
	httputil.WriteJSON(w, http.StatusCreated, h.toClinicProductResponse(r.Context(), product))
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
		httputil.WriteJSON(w, http.StatusOK, []clinicProductResponse{h.toClinicProductResponse(r.Context(), product)})
		return
	}

	products, err := h.clinicProductRepo.FindByFacility(r.Context(), facilityID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	res := make([]clinicProductResponse, 0, len(products))
	for _, p := range products {
		res = append(res, h.toClinicProductResponse(r.Context(), p))
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}
