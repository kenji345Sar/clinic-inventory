// Package distributorportal は卸ポータル向けのHTTP受け口。
//
// 卸業者担当者が自社の受注状況・商品マスタを見るための読み取り中心のAPI。
// クリニック側(/api/facilities/...)とは別フロントエンド(distributor-portal)から叩かれる想定で、
// 卸業者にはまだAuth0アカウントが無い(docs/requirements.md 8章「後続」)ため、
// このハンドラは cmd/api/main.go で RequireAuth を通さない別ルートに登録する(未認証)。
// 既存の /api/distributors 系エンドポイントの認可方針には手を付けず、
// ここは新規の /api/portal/... 配下にのみ生える。
package distributorportal

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	distapp "clinic-inventory/internal/application/distributorcatalog"
	distdomain "clinic-inventory/internal/domain/distributorcatalog"
	orgdomain "clinic-inventory/internal/domain/organization"
	procdomain "clinic-inventory/internal/domain/procurement"
	proddomain "clinic-inventory/internal/domain/productcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
	"clinic-inventory/internal/handler/httputil"
)

type Handler struct {
	registerProduct   *distapp.RegisterDistributorProductUseCase
	distributorRepo   distdomain.DistributorRepository
	productRepo       distdomain.DistributorProductRepository
	purchaseOrderRepo procdomain.PurchaseOrderRepository
	clinicProductRepo proddomain.ClinicProductRepository
	facilityRepo      orgdomain.FacilityRepository
	facilityPriceRepo distdomain.FacilityPriceRepository
}

func New(
	registerProduct *distapp.RegisterDistributorProductUseCase,
	distributorRepo distdomain.DistributorRepository,
	productRepo distdomain.DistributorProductRepository,
	purchaseOrderRepo procdomain.PurchaseOrderRepository,
	clinicProductRepo proddomain.ClinicProductRepository,
	facilityRepo orgdomain.FacilityRepository,
	facilityPriceRepo distdomain.FacilityPriceRepository,
) *Handler {
	return &Handler{
		registerProduct:   registerProduct,
		distributorRepo:   distributorRepo,
		productRepo:       productRepo,
		purchaseOrderRepo: purchaseOrderRepo,
		clinicProductRepo: clinicProductRepo,
		facilityRepo:      facilityRepo,
		facilityPriceRepo: facilityPriceRepo,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/portal/distributors", h.listDistributors)
	mux.HandleFunc("GET /api/portal/distributors/{distributorId}/products", h.listProducts)
	mux.HandleFunc("POST /api/portal/distributors/{distributorId}/products", h.postProduct)
	mux.HandleFunc("GET /api/portal/distributors/{distributorId}/products/{productId}/facility-prices", h.listFacilityPrices)
	mux.HandleFunc("GET /api/portal/distributors/{distributorId}/orders", h.listOrders)
	mux.HandleFunc("GET /api/portal/distributors/{distributorId}/orders/{orderId}", h.getOrder)
}

type distributorResponse struct {
	ID string `json:"id"`
	// Code は卸コード。S3のフォルダ名(orders/{code}/, catalogs/{code}/)に使う。
	Code string `json:"code"`
	Name string `json:"name"`
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
	// UnitPrice は卸の標準単価。nullは全医院共通の定価が無いことを表す
	// (医院ごとに単価を決めている卸・単価を提供しない卸)。
	UnitPrice *int `json:"unitPrice"`
	// FacilityPriceCount はこの商品に医院別単価が設定されている医院数。
	// 一覧で「医院別(N院)」と出すための件数で、単価そのものは内訳APIで取得する。
	FacilityPriceCount int  `json:"facilityPriceCount"`
	Discontinued       bool `json:"discontinued"`
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

	// 医院別単価は件数だけを一括で数える。商品数(1社数千件)×医院数の単価を
	// 一覧で毎回返すと重くなるため、内訳は商品を選んだときに別途取得する。
	productIDs := make([]shareddomain.ID, 0, len(products))
	for _, p := range products {
		productIDs = append(productIDs, p.ID())
	}
	counts, err := h.facilityPriceRepo.CountByProducts(r.Context(), productIDs)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	res := make([]distributorProductResponse, 0, len(products))
	for _, p := range products {
		item := toDistributorProductResponse(p)
		item.FacilityPriceCount = counts[p.ID()]
		res = append(res, item)
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}

type facilityPriceResponse struct {
	FacilityID   string `json:"facilityId"`
	FacilityName string `json:"facilityName"`
	UnitPrice    int    `json:"unitPrice"`
}

// listFacilityPrices は1商品の医院別単価を医院名付きで返す。
// 卸が自社で設定した単価を確認するための画面用で、他社の情報は含まれない
// (医院別単価はその卸の商品にぶら下がっているため)。
func (h *Handler) listFacilityPrices(w http.ResponseWriter, r *http.Request) {
	distributorID, err := httputil.ParseID(r.PathValue("distributorId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	productID, err := httputil.ParseID(r.PathValue("productId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	// URLの卸IDと商品の所属卸が一致するか確認する(他社商品の単価を覗けないようにする)。
	product, err := h.productRepo.FindByID(r.Context(), productID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	if product.DistributorID() != distributorID {
		httputil.WriteError(w, fmt.Errorf("商品が見つかりません: %w", shareddomain.ErrNotFound))
		return
	}

	prices, err := h.facilityPriceRepo.FindByProduct(r.Context(), productID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	res := make([]facilityPriceResponse, 0, len(prices))
	for _, price := range prices {
		item := facilityPriceResponse{
			FacilityID: price.FacilityID().String(),
			UnitPrice:  price.UnitPrice(),
		}
		if facility, err := h.facilityRepo.FindByID(r.Context(), price.FacilityID()); err == nil {
			item.FacilityName = facility.Name()
		}
		res = append(res, item)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].FacilityName < res[j].FacilityName })
	httputil.WriteJSON(w, http.StatusOK, res)
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

type orderLineResponse struct {
	ClinicProductID        string `json:"clinicProductId"`
	ClinicProductCode      string `json:"clinicProductCode"`
	ClinicProductName      string `json:"clinicProductName"`
	DistributorProductID   string `json:"distributorProductId"`
	DistributorProductCode string `json:"distributorProductCode"`
	DistributorProductName string `json:"distributorProductName"`
	Quantity               int    `json:"quantity"`
	UnitPrice              int    `json:"unitPrice"`
	Amount                 int    `json:"amount"`
}

type orderResponse struct {
	ID            string              `json:"id"`
	FacilityID    string              `json:"facilityId"`
	FacilityName  string              `json:"facilityName"`
	DistributorID string              `json:"distributorId"`
	Status        string              `json:"status"`
	Lines         []orderLineResponse `json:"lines"`
	TotalAmount   int                 `json:"totalAmount"`
}

// toOrderResponse は発注(クリニック商品IDのみを持つ)を、卸ポータルで表示できる形に組み立てる。
// クリニック名・クリニック商品名・そのクリニック商品が指す卸商品(=卸から見た自社商品)を
// 他集約から解決してレスポンスに載せる。これは表示専用の合成であり、書き込み系の
// ユースケースとは異なりドメイン層には置かない。
func (h *Handler) toOrderResponse(ctx context.Context, o *procdomain.PurchaseOrder) orderResponse {
	facilityName := ""
	if facility, err := h.facilityRepo.FindByID(ctx, o.FacilityID()); err == nil {
		facilityName = facility.Name()
	}

	lines := make([]orderLineResponse, 0, len(o.Lines()))
	for _, l := range o.Lines() {
		line := orderLineResponse{
			ClinicProductID: l.ClinicProductID().String(),
			Quantity:        l.Quantity(),
			UnitPrice:       l.UnitPrice(),
			Amount:          l.Amount(),
		}
		if clinicProduct, err := h.clinicProductRepo.FindByID(ctx, l.ClinicProductID()); err == nil {
			line.ClinicProductCode = clinicProduct.ProductCode()
			line.ClinicProductName = clinicProduct.Name()
			if distProduct, err := h.productRepo.FindByID(ctx, clinicProduct.DistributorProductID()); err == nil {
				line.DistributorProductID = distProduct.ID().String()
				line.DistributorProductCode = distProduct.DistributorProductCode()
				line.DistributorProductName = distProduct.Name()
			}
		}
		lines = append(lines, line)
	}

	return orderResponse{
		ID:            o.ID().String(),
		FacilityID:    o.FacilityID().String(),
		FacilityName:  facilityName,
		DistributorID: o.DistributorID().String(),
		Status:        string(o.Status()),
		Lines:         lines,
		TotalAmount:   o.TotalAmount(),
	}
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	distributorID, err := httputil.ParseID(r.PathValue("distributorId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	orders, err := h.purchaseOrderRepo.FindByDistributor(r.Context(), distributorID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	res := make([]orderResponse, 0, len(orders))
	for _, o := range orders {
		res = append(res, h.toOrderResponse(r.Context(), o))
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	distributorID, err := httputil.ParseID(r.PathValue("distributorId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	orderID, err := httputil.ParseID(r.PathValue("orderId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	order, err := h.purchaseOrderRepo.FindByID(r.Context(), orderID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	// 他の卸業者宛の発注URLを直接叩かれても見えないようにする(未認証エンドポイントのため最低限のガード)。
	if order.DistributorID() != distributorID {
		httputil.WriteError(w, shareddomain.ErrNotFound)
		return
	}
	// 下書き（カートの中身）はまだ卸に届いていないので、IDを直接叩かれても見せない。
	if order.Status() != procdomain.StatusConfirmed {
		httputil.WriteError(w, shareddomain.ErrNotFound)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, h.toOrderResponse(r.Context(), order))
}
