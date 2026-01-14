package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"maintainerd/db"
	"maintainerd/model"

	"gorm.io/gorm"
)

const defaultDBPath = "/data/maintainers.db"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dbDriver := envOr("MD_DB_DRIVER", "sqlite")
	dbDSN := envOr("MD_DB_DSN", "")
	dbPath := envOr("MD_DB_PATH", defaultDBPath)
	if dbDriver == "postgres" && dbDSN == "" {
		log.Fatal("MD_DB_DSN is required when MD_DB_DRIVER=postgres")
	}
	dsn := dbPath
	if dbDriver == "postgres" {
		dsn = dbDSN
	}

	dbConn, err := db.OpenGorm(dbDriver, dsn, &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}
	store := db.NewSQLStore(dbConn)

	if err := migrate(ctx, store); err != nil {
		log.Fatalf("migrate failed: %v", err)
	}

	log.Println("migration complete")
}

func migrate(_ context.Context, store *db.SQLStore) error {
	return store.DB().AutoMigrate(
		&model.AuditLog{},
		&model.Collaborator{},
		&model.Company{},
		&model.Foundation{},
		&model.FoundationOfficer{},
		&model.StaffMember{},
		&model.Maintainer{},
		&model.MaintainerProject{},
		&model.Project{},
		&model.Service{},
		&model.ServiceTeam{},
		&model.ServiceUser{},
		&model.MaintainerRefCache{},
	)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
