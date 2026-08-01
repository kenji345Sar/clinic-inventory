package main

import (
	"context"
	"fmt"
	"log"
	"os"

	distapp "clinic-inventory/internal/application/distributorcatalog"
	orgapp "clinic-inventory/internal/application/organization"
	prodapp "clinic-inventory/internal/application/productcatalog"
	shareddomain "clinic-inventory/internal/domain/shared"
	"clinic-inventory/internal/infrastructure/database"
	distinfra "clinic-inventory/internal/infrastructure/distributorcatalog"
	orginfra "clinic-inventory/internal/infrastructure/organization"
	prodinfra "clinic-inventory/internal/infrastructure/productcatalog"

	"gorm.io/gorm"
)

func dsn() string {
	if v := os.Getenv("DATABASE_DSN"); v != "" {
		return v
	}
	return "host=localhost user=apple dbname=clinic_inventory port=5432 sslmode=disable"
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

	ctx := context.Background()
	facilityID := runOrganizationDemo(ctx, db)
	distributorProductID := runDistributorCatalogDemo(ctx, db)
	runProductCatalogDemo(ctx, db, facilityID, distributorProductID)
}

// runOrganizationDemo は組織コンテキストの縦スライス（法人・クリニック作成→DB読み出し）を実行する。
func runOrganizationDemo(ctx context.Context, db *gorm.DB) shareddomain.ID {
	corporationRepo := orginfra.NewCorporationRepository(db)
	facilityRepo := orginfra.NewFacilityRepository(db)

	createCorporation := orgapp.NewCreateCorporationUseCase(corporationRepo)
	createFacility := orgapp.NewCreateFacilityUseCase(facilityRepo, corporationRepo)

	corporation, err := createCorporation.Execute(ctx, "サンプル動物病院グループ")
	if err != nil {
		log.Fatalf("failed to create corporation: %v", err)
	}
	fmt.Printf("created corporation: id=%s name=%s\n", corporation.ID(), corporation.Name())

	facility, err := createFacility.Execute(ctx, orgapp.CreateFacilityInput{
		Name:          "サンプル動物病院 本院",
		FacilityType:  "vet",
		CorporationID: corporation.ID(),
	})
	if err != nil {
		log.Fatalf("failed to create facility: %v", err)
	}
	fmt.Printf("created facility: id=%s name=%s corporationID=%s\n", facility.ID(), facility.Name(), facility.CorporationID())

	found, err := facilityRepo.FindByID(ctx, facility.ID())
	if err != nil {
		log.Fatalf("failed to find facility: %v", err)
	}
	fmt.Printf("found facility from db: id=%s name=%s\n", found.ID(), found.Name())
	return facility.ID()
}

// runDistributorCatalogDemo は卸連携コンテキストの縦スライスを実行する。
// 卸業者作成→卸商品2件登録→同一卸商品コードの重複登録がユースケースで弾かれることを確認する。
func runDistributorCatalogDemo(ctx context.Context, db *gorm.DB) shareddomain.ID {
	distributorRepo := distinfra.NewDistributorRepository(db)
	productRepo := distinfra.NewDistributorProductRepository(db)

	createDistributor := distapp.NewCreateDistributorUseCase(distributorRepo)
	registerProduct := distapp.NewRegisterDistributorProductUseCase(productRepo)

	distributor, err := createDistributor.Execute(ctx, "サンプル医薬品卸")
	if err != nil {
		log.Fatalf("failed to create distributor: %v", err)
	}
	fmt.Printf("created distributor: id=%s name=%s\n", distributor.ID(), distributor.Name())

	p1, err := registerProduct.Execute(ctx, distapp.RegisterDistributorProductInput{
		DistributorID:          distributor.ID(),
		DistributorProductCode: "D-0001",
		Name:                   "サンプル抗生剤 100mg 100錠",
		VendorName:             "サンプル製薬",
		VendorProductCode:      "V-9001",
		JANCode:                "4900000000001",
	})
	if err != nil {
		log.Fatalf("failed to register product: %v", err)
	}
	fmt.Printf("registered product: id=%s code=%s name=%s\n", p1.ID(), p1.DistributorProductCode(), p1.Name())

	p2, err := registerProduct.Execute(ctx, distapp.RegisterDistributorProductInput{
		DistributorID:          distributor.ID(),
		DistributorProductCode: "D-0002",
		Name:                   "サンプル消炎鎮痛剤 50mg 50錠",
		VendorName:             "サンプル製薬",
	})
	if err != nil {
		log.Fatalf("failed to register product: %v", err)
	}
	fmt.Printf("registered product: id=%s code=%s name=%s\n", p2.ID(), p2.DistributorProductCode(), p2.Name())

	// 同一卸業者×同一卸商品コードの重複登録はユースケースの存在チェックで弾かれる
	if _, err := registerProduct.Execute(ctx, distapp.RegisterDistributorProductInput{
		DistributorID:          distributor.ID(),
		DistributorProductCode: "D-0001",
		Name:                   "重複コードの商品",
		VendorName:             "サンプル製薬",
	}); err != nil {
		fmt.Printf("duplicate code rejected as expected: %v\n", err)
	} else {
		log.Fatal("duplicate product code was NOT rejected — unique check is broken")
	}

	products, err := productRepo.FindByDistributor(ctx, distributor.ID())
	if err != nil {
		log.Fatalf("failed to list products: %v", err)
	}
	fmt.Printf("products of distributor %s: %d items\n", distributor.Name(), len(products))
	return p1.ID()
}

// runProductCatalogDemo は商品マスタコンテキストの縦スライスを実行する。
// 卸商品を元にクリニック商品を登録→JANの引き継ぎ確認→商品コード重複が弾かれることを確認する。
func runProductCatalogDemo(ctx context.Context, db *gorm.DB, facilityID, distributorProductID shareddomain.ID) {
	clinicProductRepo := prodinfra.NewClinicProductRepository(db)
	distributorProductRepo := distinfra.NewDistributorProductRepository(db)

	registerClinicProduct := prodapp.NewRegisterClinicProductUseCase(clinicProductRepo, distributorProductRepo)

	// 商品名・JANを指定せず登録 → 卸商品から引き継がれる
	cp, err := registerClinicProduct.Execute(ctx, prodapp.RegisterClinicProductInput{
		FacilityID:           facilityID,
		ProductCode:          "C-0001",
		DistributorProductID: distributorProductID,
		ReorderPoint:         10,
	})
	if err != nil {
		log.Fatalf("failed to register clinic product: %v", err)
	}
	fmt.Printf("registered clinic product: id=%s code=%s name=%s jan=%s reorderPoint=%d\n",
		cp.ID(), cp.ProductCode(), cp.Name(), cp.JANCode(), cp.ReorderPoint())

	// 同一クリニック×同一商品コードの重複登録はユースケースの存在チェックで弾かれる
	if _, err := registerClinicProduct.Execute(ctx, prodapp.RegisterClinicProductInput{
		FacilityID:           facilityID,
		ProductCode:          "C-0001",
		DistributorProductID: distributorProductID,
		ReorderPoint:         5,
	}); err != nil {
		fmt.Printf("duplicate clinic product code rejected as expected: %v\n", err)
	} else {
		log.Fatal("duplicate clinic product code was NOT rejected — unique check is broken")
	}

	// バーコード消費の入口となるJAN引き当ての確認
	foundByJAN, err := clinicProductRepo.FindByFacilityAndJAN(ctx, facilityID, cp.JANCode())
	if err != nil {
		log.Fatalf("failed to find clinic product by JAN: %v", err)
	}
	fmt.Printf("found clinic product by JAN %s: code=%s name=%s\n", cp.JANCode(), foundByJAN.ProductCode(), foundByJAN.Name())
}
