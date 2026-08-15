package distributorcatalog

import (
	"net/http"

	distapp "clinic-inventory/internal/application/distributorcatalog"
	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
	"clinic-inventory/internal/handler/httputil"
)

// Handler は卸連携コンテキストのHTTP受け口。
type Handler struct {
	createDistributor *distapp.CreateDistributorUseCase
	registerProduct   *distapp.RegisterDistributorProductUseCase
	distributorRepo   distdomain.DistributorRepository
	productRepo       distdomain.DistributorProductRepository
	facilityPriceRepo distdomain.FacilityPriceRepository
}

func New(
	createDistributor *distapp.CreateDistributorUseCase,
	registerProduct *distapp.RegisterDistributorProductUseCase,
	distributorRepo distdomain.DistributorRepository,
	productRepo distdomain.DistributorProductRepository,
	facilityPriceRepo distdomain.FacilityPriceRepository,
) *Handler {
	return &Handler{
		createDistributor: createDistributor,
		registerProduct:   registerProduct,
		distributorRepo:   distributorRepo,
		productRepo:       productRepo,
		facilityPriceRepo: facilityPriceRepo,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/distributors", h.postDistributor)
	mux.HandleFunc("GET /api/distributors", h.listDistributors)
	mux.HandleFunc("POST /api/distributors/{distributorId}/products", h.postProduct)
	mux.HandleFunc("GET /api/distributors/{distributorId}/products", h.listProducts)
}

type distributorResponse struct {
	ID string `json:"id"`
	// Code は卸コード。S3のフォルダ名(orders/{code}/, catalogs/{code}/)に使う。
	Code string `json:"code"`
	Name string `json:"name"`
}

func (h *Handler) postDistributor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, err)
		return
	}
	distributor, err := h.createDistributor.Execute(r.Context(), distapp.CreateDistributorInput{
		Code: req.Code,
		Name: req.Name,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, distributorResponse{
		ID:   distributor.ID().String(),
		Code: distributor.Code(),
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
		res = append(res, distributorResponse{ID: d.ID().String(), Code: d.Code(), Name: d.Name()})
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
	// UnitPrice は卸の標準単価。nullは卸が単価を公表していないことを表す。
	UnitPrice *int `json:"unitPrice"`
	// FacilityUnitPrice は、クエリでfacilityIdを指定した場合に入るそのクリニック向けの単価
	// (医院ごとに単価を決めている卸)。指定が無い・設定が無い場合はnull。
	FacilityUnitPrice *int `json:"facilityUnitPrice"`
	Discontinued      bool `json:"discontinued"`
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
		UnitPrice:              p.UnitPrice(),
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
		UnitPrice              *int   `json:"unitPrice"` // 未指定(null)は単価非公表
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
		UnitPrice:              req.UnitPrice,
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

	// クエリでfacilityIdが指定されていれば、そのクリニック向けの医院別単価を併せて返す。
	// クリニックが商品を選ぶ画面で「自院の契約単価」を出すために使う。
	// 商品数分の問い合わせにならないよう、クリニック単位で一括取得してから突き合わせる。
	facilityPrices := map[shareddomain.ID]int{}
	if raw := r.URL.Query().Get("facilityId"); raw != "" {
		facilityID, err := httputil.ParseID(raw)
		if err != nil {
			httputil.WriteError(w, err)
			return
		}
		prices, err := h.facilityPriceRepo.FindByFacility(r.Context(), facilityID)
		if err != nil {
			httputil.WriteError(w, err)
			return
		}
		for _, price := range prices {
			facilityPrices[price.DistributorProductID()] = price.UnitPrice()
		}
	}

	res := make([]distributorProductResponse, 0, len(products))
	for _, p := range products {
		item := toDistributorProductResponse(p)
		if price, ok := facilityPrices[p.ID()]; ok {
			item.FacilityUnitPrice = &price
		}
		res = append(res, item)
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}
