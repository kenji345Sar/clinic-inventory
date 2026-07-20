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

	// ハンドラ
	mux := http.NewServeMux()
	orghandler.New(createCorporation, createFacility, facilityRepo).Register(mux)
	disthandler.New(createDistributor, registerDistributorProduct, distributorRepo, distributorProductRepo).Register(mux)
	prodhandler.New(registerClinicProduct, clinicProductRepo, distributorProductRepo, distributorRepo).Register(mux)
	prochandler.New(createPurchaseOrder, purchaseOrderRepo).Register(mux)

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	addr := ":" + port()
	fmt.Printf("clinic-inventory api listening on %s\n", addr)
	if err := http.ListenAndServe(addr, httputil.CORS(mux)); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
