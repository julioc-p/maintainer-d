package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"maintainerd/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dbPath := flag.String("db", "testdata/maintainerd_test.db", "Path to SQLite database file")
	flag.Parse()

	db, err := gorm.Open(sqlite.Open(*dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("seed: failed to open db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Company{},
		&model.Foundation{},
		&model.Project{},
		&model.Maintainer{},
		&model.StaffMember{},
		&model.FoundationOfficer{},
		&model.Collaborator{},
		&model.MaintainerProject{},
		&model.Service{},
		&model.ServiceTeam{},
		&model.ServiceUser{},
		&model.ServiceUserTeams{},
	); err != nil {
		log.Fatalf("seed: auto-migrate failed: %v", err)
	}

	company := model.Company{Name: "Example Labs"}
	foundation := model.Foundation{Name: "CNCF"}
	if err := db.FirstOrCreate(&company, model.Company{Name: company.Name}).Error; err != nil {
		log.Fatalf("seed: company insert failed: %v", err)
	}
	if err := db.FirstOrCreate(&foundation, model.Foundation{Name: foundation.Name}).Error; err != nil {
		log.Fatalf("seed: foundation insert failed: %v", err)
	}

	staff := model.StaffMember{
		Name:          "Staff Tester",
		Email:         "staff.tester@example.test",
		GitHubAccount: "staff-tester",
		GitHubEmail:   "staff.tester@github.test",
		FoundationID:  &foundation.ID,
		RegisteredAt:  timePtr(time.Now()),
	}
	if err := db.FirstOrCreate(&staff, model.StaffMember{GitHubAccount: staff.GitHubAccount}).Error; err != nil {
		log.Fatalf("seed: staff insert failed: %v", err)
	}

	maintainers := []model.Maintainer{
		{
			Name:             "Antonio Example",
			Email:            "antonio.example@test.dev",
			GitHubAccount:    "antonio-example",
			GitHubEmail:      "antonio@example.dev",
			MaintainerStatus: model.ActiveMaintainer,
			CompanyID:        &company.ID,
			RegisteredAt:     timePtr(time.Now()),
		},
		{
			Name:             "Renee Sample",
			Email:            "renee.sample@test.dev",
			GitHubAccount:    "renee-sample",
			GitHubEmail:      "renee@example.dev",
			MaintainerStatus: model.ActiveMaintainer,
			CompanyID:        &company.ID,
			RegisteredAt:     timePtr(time.Now()),
		},
		{
			Name:             "Diego Placeholder",
			Email:            "diego.placeholder@test.dev",
			GitHubAccount:    "diego-placeholder",
			GitHubEmail:      "diego@example.dev",
			MaintainerStatus: model.ActiveMaintainer,
			CompanyID:        &company.ID,
			RegisteredAt:     timePtr(time.Now()),
		},
		{
			Name:             "Jun Example",
			Email:            "jun.example@test.dev",
			GitHubAccount:    "jun-example",
			GitHubEmail:      "jun@example.dev",
			MaintainerStatus: model.ActiveMaintainer,
			CompanyID:        &company.ID,
			RegisteredAt:     timePtr(time.Now()),
		},
		{
			Name:             "Priya Demo",
			Email:            "priya.demo@test.dev",
			GitHubAccount:    "priya-demo",
			GitHubEmail:      "priya@example.dev",
			MaintainerStatus: model.ActiveMaintainer,
			CompanyID:        &company.ID,
			RegisteredAt:     timePtr(time.Now()),
		},
	}

	for i := range maintainers {
		if err := db.FirstOrCreate(&maintainers[i], model.Maintainer{GitHubAccount: maintainers[i].GitHubAccount}).Error; err != nil {
			log.Fatalf("seed: maintainer insert failed: %v", err)
		}
	}

	projects := []model.Project{
		{Name: "Project Atlas", Maturity: model.Graduated},
		{Name: "Project Beacon", Maturity: model.Incubating},
		{Name: "Project Comet", Maturity: model.Sandbox},
	}

	for i := range projects {
		if err := db.FirstOrCreate(&projects[i], model.Project{Name: projects[i].Name}).Error; err != nil {
			log.Fatalf("seed: project insert failed: %v", err)
		}
	}

	if err := db.Model(&projects[0]).Association("Maintainers").Replace(
		&maintainers[0],
		&maintainers[1],
	); err != nil {
		log.Fatalf("seed: association failed: %v", err)
	}
	if err := db.Model(&projects[1]).Association("Maintainers").Replace(
		&maintainers[0],
		&maintainers[2],
	); err != nil {
		log.Fatalf("seed: association failed: %v", err)
	}
	if err := db.Model(&projects[2]).Association("Maintainers").Replace(
		&maintainers[0],
		&maintainers[3],
		&maintainers[4],
	); err != nil {
		log.Fatalf("seed: association failed: %v", err)
	}

	fmt.Printf("seed: wrote test db to %s\n", *dbPath)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
