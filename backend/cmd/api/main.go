package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	distapp "clinic-inventory/internal/application/distributorcatalog"
	orgapp "clinic-inventory/internal/application/organization"
	prodapp "clinic-inventory/internal/application/productcatalog"
	disthandler "clinic-inventory/internal/handler/distributorcatalog"
	"clinic-inventory/internal/handler/httputil"
	orghandler "clinic-inventory/internal/handler/organization"
	prodhandler "clinic-inventory/internal/handler/productcatalog"
	"clinic-inventory/internal/infrastructure/database"
	distinfra "clinic-inventory/internal/infrastructure/distributorcatalog"
	orginfra "clinic-inventory/internal/infrastructure/organization"
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
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	// リポジトリ
	corporationRepo := orginfra.NewCorporationRepository(db)
	facilityRepo := orginfra.NewFacilityRepository(db)
	distributorRepo := distinfra.NewDistributorRepository(db)
	distributorProductRepo := distinfra.NewDistributorProductRepository(db)
	clinicProductRepo := prodinfra.NewClinicProductRepository(db)

	// ユースケース
	createCorporation := orgapp.NewCreateCorporationUseCase(corporationRepo)
	createFacility := orgapp.NewCreateFacilityUseCase(facilityRepo)
	createDistributor := distapp.NewCreateDistributorUseCase(distributorRepo)
	registerDistributorProduct := distapp.NewRegisterDistributorProductUseCase(distributorProductRepo)
	registerClinicProduct := prodapp.NewRegisterClinicProductUseCase(clinicProductRepo, distributorProductRepo)

	// ハンドラ
	mux := http.NewServeMux()
	orghandler.New(createCorporation, createFacility, facilityRepo).Register(mux)
	disthandler.New(createDistributor, registerDistributorProduct, distributorRepo, distributorProductRepo).Register(mux)
	prodhandler.New(registerClinicProduct, clinicProductRepo).Register(mux)

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	addr := ":" + port()
	fmt.Printf("clinic-inventory api listening on %s\n", addr)
	if err := http.ListenAndServe(addr, httputil.CORS(mux)); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
