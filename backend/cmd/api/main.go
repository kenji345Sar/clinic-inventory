package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	distapp "clinic-inventory/internal/application/distributorcatalog"
	orgapp "clinic-inventory/internal/application/organization"
	procapp "clinic-inventory/internal/application/procurement"
	prodapp "clinic-inventory/internal/application/productcatalog"
	disthandler "clinic-inventory/internal/handler/distributorcatalog"
	"clinic-inventory/internal/handler/httputil"
	orghandler "clinic-inventory/internal/handler/organization"
	prochandler "clinic-inventory/internal/handler/procurement"
	prodhandler "clinic-inventory/internal/handler/productcatalog"
	"clinic-inventory/internal/infrastructure/database"
	distinfra "clinic-inventory/internal/infrastructure/distributorcatalog"
	orginfra "clinic-inventory/internal/infrastructure/organization"
	procinfra "clinic-inventory/internal/infrastructure/procurement"
	prodinfra "clinic-inventory/internal/infrastructure/productcatalog"
)

func dsn() string {
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		return v
	}
	return "host=localhost user=apple dbname=clinic_inventory port=5432 sslmode=disable"
}

func port() string {
	if v := os.Getenv("PORT"); v != "" {
		return v
	}
	return "8080"
}

func main() {
	db, err := database.Connect(dsn())
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(
		&orginfra.CorporationModel{},
		&orginfra.FacilityModel{},
		&distinfra.DistributorModel{},
		&distinfra.DistributorProductModel{},
		&prodinfra.ClinicProductModel{},
		&procinfra.PurchaseOrderModel{},
		&procinfra.PurchaseOrderLineModel{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	// リポジトリ
	corporationRepo := orginfra.NewCorporationRepository(db)
	facilityRepo := orginfra.NewFacilityRepository(db)
	distributorRepo := distinfra.NewDistributorRepository(db)
	distributorProductRepo := distinfra.NewDistributorProductRepository(db)
	clinicProductRepo := prodinfra.NewClinicProductRepository(db)
	purchaseOrderRepo := procinfra.NewPurchaseOrderRepository(db)

	// ユースケース
	createCorporation := orgapp.NewCreateCorporationUseCase(corporationRepo)
	createFacility := orgapp.NewCreateFacilityUseCase(facilityRepo)
	createDistributor := distapp.NewCreateDistributorUseCase(distributorRepo)
	registerDistributorProduct := distapp.NewRegisterDistributorProductUseCase(distributorProductRepo)
	registerClinicProduct := prodapp.NewRegisterClinicProductUseCase(clinicProductRepo, distributorProductRepo)
	createPurchaseOrder := procapp.NewCreatePurchaseOrderUseCase(purchaseOrderRepo, distributorRepo, distributorProductRepo, clinicProductRepo)

	// 認証が必要な業務APIハンドラ
	protected := http.NewServeMux()
	orghandler.New(createCorporation, createFacility, facilityRepo).Register(protected)
	disthandler.New(createDistributor, registerDistributorProduct, distributorRepo, distributorProductRepo).Register(protected)
	prodhandler.New(registerClinicProduct, clinicProductRepo, distributorProductRepo, distributorRepo).Register(protected)
	prochandler.New(createPurchaseOrder, purchaseOrderRepo).Register(protected)

	// ルーティング。health は認証不要、それ以外の /api/* は RequireAuth を通す。
	// Go 1.22 の ServeMux は "GET /api/health" を "/api/" より具体的として優先するため、
	// health だけ素通しし、残りの業務APIは protected(認証必須)へ委譲される。
	root := http.NewServeMux()
	root.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	root.Handle("/api/", httputil.RequireAuth(protected))

	addr := ":" + port()
	fmt.Printf("clinic-inventory api listening on %s\n", addr)
	if err := http.ListenAndServe(addr, httputil.CORS(root)); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
