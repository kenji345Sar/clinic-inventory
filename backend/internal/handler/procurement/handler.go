package procurement

import (
	"net/http"

	procapp "clinic-inventory/internal/application/procurement"
	procdomain "clinic-inventory/internal/domain/procurement"
	"clinic-inventory/internal/handler/httputil"
)

// Handler は発注コンテキストのHTTP受け口。
type Handler struct {
	createOrder       *procapp.CreatePurchaseOrderUseCase
	purchaseOrderRepo procdomain.PurchaseOrderRepository
}

func New(
	createOrder *procapp.CreatePurchaseOrderUseCase,
	purchaseOrderRepo procdomain.PurchaseOrderRepository,
) *Handler {
	return &Handler{
		createOrder:       createOrder,
		purchaseOrderRepo: purchaseOrderRepo,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/facilities/{facilityId}/orders", h.postOrder)
	mux.HandleFunc("GET /api/facilities/{facilityId}/orders", h.listOrders)
	mux.HandleFunc("GET /api/facilities/{facilityId}/orders/{orderId}", h.getOrder)
}

type orderLineResponse struct {
	ClinicProductID string `json:"clinicProductId"`
	Quantity        int    `json:"quantity"`
	UnitPrice       int    `json:"unitPrice"`
	Amount          int    `json:"amount"`
}

type purchaseOrderResponse struct {
	ID            string              `json:"id"`
	FacilityID    string              `json:"facilityId"`
	DistributorID string              `json:"distributorId"`
	Status        string              `json:"status"`
	Lines         []orderLineResponse `json:"lines"`
	TotalAmount   int                 `json:"totalAmount"`
}

func toPurchaseOrderResponse(o *procdomain.PurchaseOrder) purchaseOrderResponse {
	lines := make([]orderLineResponse, 0, len(o.Lines()))
	for _, l := range o.Lines() {
		lines = append(lines, orderLineResponse{
			ClinicProductID: l.ClinicProductID().String(),
			Quantity:        l.Quantity(),
			UnitPrice:       l.UnitPrice(),
			Amount:          l.Amount(),
		})
	}
	return purchaseOrderResponse{
		ID:            o.ID().String(),
		FacilityID:    o.FacilityID().String(),
		DistributorID: o.DistributorID().String(),
		Status:        string(o.Status()),
		Lines:         lines,
		TotalAmount:   o.TotalAmount(),
	}
}

func (h *Handler) postOrder(w http.ResponseWriter, r *http.Request) {
	facilityID, err := httputil.ParseID(r.PathValue("facilityId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	var req struct {
		DistributorID string `json:"distributorId"`
		Lines         []struct {
			ClinicProductID string `json:"clinicProductId"`
			Quantity        int    `json:"quantity"`
		} `json:"lines"`
	}
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, err)
		return
	}
	distributorID, err := httputil.ParseID(req.DistributorID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}

	lines := make([]procapp.CreatePurchaseOrderLineInput, 0, len(req.Lines))
	for _, l := range req.Lines {
		clinicProductID, err := httputil.ParseID(l.ClinicProductID)
		if err != nil {
			httputil.WriteError(w, err)
			return
		}
		lines = append(lines, procapp.CreatePurchaseOrderLineInput{
			ClinicProductID: clinicProductID,
			Quantity:        l.Quantity,
		})
	}

	order, err := h.createOrder.Execute(r.Context(), procapp.CreatePurchaseOrderInput{
		FacilityID:    facilityID,
		DistributorID: distributorID,
		Lines:         lines,
	})
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	httputil.WriteJSON(w, http.StatusCreated, toPurchaseOrderResponse(order))
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	facilityID, err := httputil.ParseID(r.PathValue("facilityId"))
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	orders, err := h.purchaseOrderRepo.FindByFacility(r.Context(), facilityID)
	if err != nil {
		httputil.WriteError(w, err)
		return
	}
	res := make([]purchaseOrderResponse, 0, len(orders))
	for _, o := range orders {
		res = append(res, toPurchaseOrderResponse(o))
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
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
	httputil.WriteJSON(w, http.StatusOK, toPurchaseOrderResponse(order))
}
