package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"maintainerd/db"
	"maintainerd/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	ctx := context.Background()
	container, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("maintainerd_test"),
		postgres.WithUsername("maintainerd"),
		postgres.WithPassword("maintainerd"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	gormDB, err := db.OpenGorm("postgres", dsn, &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	err = gormDB.AutoMigrate(
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
	)
	require.NoError(t, err)

	return gormDB
}

func performMaintainerGet(t *testing.T, s *server, maintainerID uint, sessionID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/maintainers/%d", maintainerID), nil)
	req.AddCookie(&http.Cookie{Name: s.cookieName, Value: sessionID})
	rec := httptest.NewRecorder()
	handler := s.requireSession(http.HandlerFunc(s.handleMaintainer))
	handler.ServeHTTP(rec, req)
	return rec
}

func TestMaintainerEmailRedaction(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now()

	company := model.Company{Name: "Test Co"}
	require.NoError(t, dbConn.Create(&company).Error)

	project := model.Project{Name: "Project A", Maturity: model.Sandbox}
	require.NoError(t, dbConn.Create(&project).Error)

	alice := model.Maintainer{
		Name:             "Alice Example",
		Email:            "alice@example.org",
		GitHubAccount:    "alice-example",
		GitHubEmail:      "alice@github.example",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	require.NoError(t, dbConn.Create(&alice).Error)

	bob := model.Maintainer{
		Name:             "Bob Example",
		Email:            "bob@example.org",
		GitHubAccount:    "bob-example",
		GitHubEmail:      "bob@github.example",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	require.NoError(t, dbConn.Create(&bob).Error)

	require.NoError(t, dbConn.Model(&project).Association("Maintainers").Append(&alice, &bob))

	staff := model.StaffMember{
		Name:          "Staff Tester",
		GitHubAccount: "staff-tester",
		Email:         "staff@example.org",
	}
	require.NoError(t, dbConn.Create(&staff).Error)

	s := &server{
		store:      store,
		sessions:   newSessionStore(log.New(io.Discard, "", 0)),
		cookieName: defaultSessionCookieName,
		logger:     log.New(io.Discard, "", 0),
	}

	staffSessionID := "staff-session"
	s.sessions.Set(session{
		ID:        staffSessionID,
		Login:     staff.GitHubAccount,
		Role:      roleStaff,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	aliceSessionID := "alice-session"
	s.sessions.Set(session{
		ID:        aliceSessionID,
		Login:     alice.GitHubAccount,
		Role:      roleMaintainer,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	t.Run("staff sees all maintainer emails", func(t *testing.T) {
		rec := performMaintainerGet(t, s, bob.ID, staffSessionID)
		require.Equal(t, http.StatusOK, rec.Code)
		var response maintainerDetailResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.Equal(t, bob.Email, response.Email)
		assert.Equal(t, bob.GitHubEmail, response.GitHubEmail)
	})

	t.Run("maintainer sees own email", func(t *testing.T) {
		rec := performMaintainerGet(t, s, alice.ID, aliceSessionID)
		require.Equal(t, http.StatusOK, rec.Code)
		var response maintainerDetailResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.Equal(t, alice.Email, response.Email)
		assert.Equal(t, alice.GitHubEmail, response.GitHubEmail)
	})

	t.Run("maintainer cannot see other maintainer email", func(t *testing.T) {
		rec := performMaintainerGet(t, s, bob.ID, aliceSessionID)
		require.Equal(t, http.StatusOK, rec.Code)
		var response maintainerDetailResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.Empty(t, response.Email)
		assert.Empty(t, response.GitHubEmail)
	})
}

func TestMaintainerCanAccessAllProjectsAndMaintainers(t *testing.T) {
	dbConn := setupPostgresTestDB(t)
	store := db.NewSQLStore(dbConn)
	now := time.Now()

	company := model.Company{Name: "Test Co"}
	require.NoError(t, dbConn.Create(&company).Error)

	projectA := model.Project{Name: "Project A", Maturity: model.Sandbox}
	projectB := model.Project{Name: "Project B", Maturity: model.Graduated}
	require.NoError(t, dbConn.Create(&projectA).Error)
	require.NoError(t, dbConn.Create(&projectB).Error)

	alice := model.Maintainer{
		Name:             "Alice Example",
		Email:            "alice@example.org",
		GitHubAccount:    "alice-example",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	bob := model.Maintainer{
		Name:             "Bob Example",
		Email:            "bob@example.org",
		GitHubAccount:    "bob-example",
		MaintainerStatus: model.ActiveMaintainer,
		CompanyID:        &company.ID,
	}
	require.NoError(t, dbConn.Create(&alice).Error)
	require.NoError(t, dbConn.Create(&bob).Error)
	require.NoError(t, dbConn.Model(&projectA).Association("Maintainers").Append(&alice))

	s := &server{
		store:      store,
		sessions:   newSessionStore(log.New(io.Discard, "", 0)),
		cookieName: defaultSessionCookieName,
		logger:     log.New(io.Discard, "", 0),
	}

	maintainerSessionID := "alice-session"
	s.sessions.Set(session{
		ID:        maintainerSessionID,
		Login:     alice.GitHubAccount,
		Role:      roleMaintainer,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	})

	t.Run("maintainer can access any project", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/projects/%d", projectB.ID), nil)
		req.AddCookie(&http.Cookie{Name: s.cookieName, Value: maintainerSessionID})
		rec := httptest.NewRecorder()
		handler := s.requireSession(http.HandlerFunc(s.handleProject))
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("maintainer can list all projects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
		req.AddCookie(&http.Cookie{Name: s.cookieName, Value: maintainerSessionID})
		rec := httptest.NewRecorder()
		handler := s.requireSession(http.HandlerFunc(s.handleProjects))
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var response projectsResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&response))
		assert.GreaterOrEqual(t, response.Total, int64(2))
	})

	t.Run("maintainer can access other maintainer profiles", func(t *testing.T) {
		rec := performMaintainerGet(t, s, bob.ID, maintainerSessionID)
		require.Equal(t, http.StatusOK, rec.Code)
	})
}
