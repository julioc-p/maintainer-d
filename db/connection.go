package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// OpenGorm opens a gorm DB connection using the specified driver and DSN.
// driver: "sqlite" or "postgres"
// dsn: sqlite file path or Postgres DSN.
func OpenGorm(driver, dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	switch driver {
	case "", "sqlite":
		if dsn == "" {
			return nil, fmt.Errorf("sqlite requires a db path")
		}
		return gorm.Open(sqlite.Open(dsn), cfg)
	case "postgres":
		if dsn == "" {
			return nil, fmt.Errorf("postgres requires a DSN")
		}
		return gorm.Open(postgres.Open(dsn), cfg)
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", driver)
	}
}
