module erp-billing-service

go 1.24.0

toolchain go1.24.10

require (
	github.com/efs/shared-events v0.0.0-00010101000000-000000000000
	github.com/efs/shared-kafka v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/joho/godotenv v1.5.1
	google.golang.org/grpc v1.77.0
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.31.1
)

require (
	github.com/boombuler/barcode v1.0.1 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.12.0 // indirect
	github.com/f-amaral/go-async v0.3.0 // indirect
	github.com/hhrutter/lzw v1.0.0 // indirect
	github.com/hhrutter/tiff v1.0.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.6.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/johnfercher/go-tree v1.0.5 // indirect
	github.com/johnfercher/maroto/v2 v2.3.3 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/pdfcpu/pdfcpu v0.6.0 // indirect
	github.com/phpdave11/gofpdf v1.4.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/image v0.18.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/efs/shared-events => ../efs-shared-events

replace github.com/efs/shared-kafka => ../efs-shared-kafka
