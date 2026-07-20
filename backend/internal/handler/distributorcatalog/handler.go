package distributorcatalog

import (
	"net/http"

	distapp "clinic-inventory/internal/application/distributorcatalog"
	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	"clinic-inventory/internal/handler/httputil"
)

// Handler は卸連携コンテキストのHTTP受け口。
type Handler struct {
	createDistributor *distapp.CreateDistributorUseCase
	registerProduct   *distapp.RegisterDistributorProductUseCase
	distributorRepo   distdomain.DistributorRepository
	productRepo       distdomain.DistributorProductRepository
}

func New(
	createDistributor *distapp.CreateDistributorUseCase,
	registerProduct *distapp.RegisterDistributorProductUseCase,
	distributorRepo distdomain.DistributorRepository,
	productRepo distdomain.DistributorProductRepository,
) *Handler {
	return &Handler{
		createDistributor: createDistributor,
		registerProduct:   registerProduct,
		distributorRepo:   distributorRepo,
		productRepo:       productRepo,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/distributors", h.postDistributor)
	mux.HandleFunc("GET /api/distributors", h.listDistributors)
	mux.HandleFunc("POST /api/distributors/{distributorId}/products", h.postProduct)
	mux.HandleFunc("GET /api/distributors/{distributorId}/products", h.listProducts)
}

type distributorResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (h *Handler) postDistributor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, err)
		return
	}
	distributor, err := h.createDistributor.Execute(r.Context(), req.Name)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, distributorResponse{
		ID:   distributor.ID().String(),
		Name: distributor.Name(),
	})
}

func (h *Handler) listDistributors(w http.ResponseWriter, r *http.Request) {
	distributors, err := h.distributorRepo.FindAll(r.Context())
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	res := make([]distributorResponse, 0, len(distributors))
	for _, d := range distributors {
		res = append(res, distributorResponse{ID: d.ID().String(), Name: d.Name()})
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}

type distributorProductResponse struct {
	ID                     string `json:"id"`
	DistributorID          string `json:"distributorId"`
	DistributorProductCode string `json:"distributorProductCode"`
	Name                   string `json:"name"`
	VendorName             string `json:"vendorName"`
	VendorProductCode      string `json:"vendorProductCode"`
	JANCode                string `json:"janCode"`
	Discontinued           bool   `json:"discontinued"`
}

func toDistributorProductResponse(p *distdomain.DistributorProduct) distributorProductResponse {
	return distributorProductResponse{
		ID:                     p.ID().String(),
		DistributorID:          p.DistributorID().String(),
		DistributorProductCode: p.DistributorProductCode(),
		Name:                   p.Name(),
		VendorName:             p.VendorName(),
		VendorProductCode:      p.VendorProductCode(),
		JANCode:                p.JANCode(),
		Discontinued:           p.Discontinued(),
	}
}

func (h *Handler) postProduct(w http.ResponseWriter, r *http.Request) {
	distributorID, err := httputil.ParseID(r.PathValue("distributorId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	var req struct {
		DistributorProductCode string `json:"distributorProductCode"`
		Name                   string `json:"name"`
		VendorName             string `json:"vendorName"`
		VendorProductCode      string `json:"vendorProductCode"`
		JANCode                string `json:"janCode"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, err)
		return
	}
	product, err := h.registerProduct.Execute(r.Context(), distapp.RegisterDistributorProductInput{
		DistributorID:          distributorID,
		DistributorProductCode: req.DistributorProductCode,
		Name:                   req.Name,
		VendorName:             req.VendorName,
		VendorProductCode:      req.VendorProductCode,
		JANCode:                req.JANCode,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, toDistributorProductResponse(product))
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	distributorID, err := httputil.ParseID(r.PathValue("distributorId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	products, err := h.productRepo.FindByDistributor(r.Context(), distributorID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	res := make([]distributorProductResponse, 0, len(products))
	for _, p := range products {
		res = append(res, toDistributorProductResponse(p))
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}
