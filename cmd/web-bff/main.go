package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"maintainerd/db"
	"maintainerd/model"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
	ghoauth "golang.org/x/oauth2/github"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func redactPostgresDSN(dsn string) (string, error) {
	parts := strings.Fields(dsn)
	kv := make(map[string]string, len(parts))
	for _, part := range parts {
		split := strings.SplitN(part, "=", 2)
		if len(split) != 2 {
			continue
		}
		kv[split[0]] = split[1]
	}
	user := kv["user"]
	host := kv["host"]
	port := kv["port"]
	dbname := kv["dbname"]
	if host == "" || dbname == "" {
		return "", fmt.Errorf("missing host/dbname in DSN")
	}
	if port == "" {
		port = "5432"
	}
	return fmt.Sprintf("user=%s host=%s port=%s dbname=%s password=***", user, host, port, dbname), nil
}

const (
	defaultAddr              = ":8000"
	defaultSessionCookieName = "md_session"
	defaultStateCookieName   = "md_oauth_state"
	defaultSessionTTL        = 8 * time.Hour
	defaultStateTTL          = 10 * time.Minute
	defaultDBPath            = "/data/onboarding.db"
	defaultWebBaseURL        = "http://localhost:3000"
	defaultRedirectCallback  = "http://localhost:8000/auth/callback"
	loginRedirectParam       = "next"
	headerContentType        = "Content-Type"
	contentTypeJSON          = "application/json"
	roleStaff                = "staff"
	roleMaintainer           = "maintainer"
)

type server struct {
	oauthConfig  *oauth2.Config
	store        *db.SQLStore
	sessions     *sessionStore
	oauthStates  *stateStore
	cookieName   string
	stateCookie  string
	webBaseURL   string
	cookieDomain string
	cookieSecure bool
	sessionTTL   time.Duration
	webOrigin    string
	testMode     bool
	logger       *log.Logger
}

type session struct {
	ID        string
	Login     string
	Role      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]session
	logger   *log.Logger
}

type stateEntry struct {
	Redirect string
	Expires  time.Time
}

type stateStore struct {
	mu     sync.RWMutex
	states map[string]stateEntry
	ttl    time.Duration
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	addr := envOr("BFF_ADDR", defaultAddr)
	dbDriver := envOr("MD_DB_DRIVER", "sqlite")
	dbDSN := envOr("MD_DB_DSN", "")
	dbPath := envOr("MD_DB_PATH", defaultDBPath)
	webBaseURL := envOr("WEB_APP_BASE_URL", defaultWebBaseURL)
	redirectURL := envOr("GITHUB_OAUTH_REDIRECT_URL", defaultRedirectCallback)
	cookieName := envOr("SESSION_COOKIE_NAME", defaultSessionCookieName)
	cookieDomain := os.Getenv("SESSION_COOKIE_DOMAIN")
	stateCookie := envOr("OAUTH_STATE_COOKIE_NAME", defaultStateCookieName)
	sessionTTL := parseDuration(envOr("SESSION_TTL", ""), defaultSessionTTL)
	cookieSecure := envOr("SESSION_COOKIE_SECURE", "") == "true"
	testMode := envOr("BFF_TEST_MODE", "") == "true"

	clientID := os.Getenv("GITHUB_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_OAUTH_CLIENT_SECRET")
	if dbDriver == "sqlite" {
		logger.Printf(
			"web-bff: config addr=%s dbDriver=%s dbPath=%s webBaseURL=%s redirectURL=%s cookieName=%s cookieDomain=%s stateCookie=%s sessionTTL=%s cookieSecure=%t testMode=%t clientID=%s",
			addr,
			dbDriver,
			dbPath,
			webBaseURL,
			redirectURL,
			cookieName,
			cookieDomain,
			stateCookie,
			sessionTTL,
			cookieSecure,
			testMode,
			clientID,
		)
	} else {
		logger.Printf(
			"web-bff: config addr=%s dbDriver=%s dbDSNSet=%t webBaseURL=%s redirectURL=%s cookieName=%s cookieDomain=%s stateCookie=%s sessionTTL=%s cookieSecure=%t testMode=%t clientID=%s",
			addr,
			dbDriver,
			dbDSN != "",
			webBaseURL,
			redirectURL,
			cookieName,
			cookieDomain,
			stateCookie,
			sessionTTL,
			cookieSecure,
			testMode,
			clientID,
		)
	}
	if !testMode && (clientID == "" || clientSecret == "") {
		logger.Fatal("web-bff: GITHUB_OAUTH_CLIENT_ID and GITHUB_OAUTH_CLIENT_SECRET must be set")
	}
	if testMode && clientID == "" {
		clientID = "test-client"
	}
	if testMode && clientSecret == "" {
		clientSecret = "test-secret"
	}
	if dbDriver == "postgres" && dbDSN == "" {
		logger.Fatal("web-bff: MD_DB_DSN is required when MD_DB_DRIVER=postgres")
	}

	redirectURLParsed, err := url.Parse(redirectURL)
	if err != nil {
		logger.Fatalf("web-bff: invalid GITHUB_OAUTH_REDIRECT_URL: %v", err)
	}
	if !cookieSecure {
		cookieSecure = redirectURLParsed.Scheme == "https"
	}

	dsn := dbPath
	if dbDriver == "postgres" {
		dsn = dbDSN
	}
	if dbDriver == "postgres" && dbDSN != "" {
		logDB, err := redactPostgresDSN(dbDSN)
		if err != nil {
			logger.Printf("web-bff: using postgres DSN (failed to parse details): %v", err)
		} else {
			logger.Printf("web-bff: using postgres DSN %s", logDB)
		}
	}
	store, err := openStore(dbDriver, dsn)
	if err != nil {
		logger.Fatalf("web-bff: failed to open database: %v", err)
	}

	s := &server{
		oauthConfig: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"read:user"},
			Endpoint:     ghoauth.Endpoint,
		},
		store:        store,
		sessions:     newSessionStore(logger),
		oauthStates:  newStateStore(defaultStateTTL),
		cookieName:   cookieName,
		stateCookie:  stateCookie,
		webBaseURL:   strings.TrimRight(webBaseURL, "/"),
		cookieDomain: cookieDomain,
		cookieSecure: cookieSecure,
		sessionTTL:   sessionTTL,
		webOrigin:    originFromBaseURL(webBaseURL),
		testMode:     testMode,
		logger:       logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/auth/login", s.handleLogin)
	mux.HandleFunc("/auth/callback", s.handleCallback)
	mux.Handle("/auth/test-login", s.withCORS(http.HandlerFunc(s.handleTestLogin)))
	mux.Handle("/auth/logout", s.withCORS(http.HandlerFunc(s.handleLogout)))
	mux.Handle("/api/me", s.withCORS(s.requireSession(http.HandlerFunc(s.handleMe))))
	mux.Handle("/api/projects", s.withCORS(s.requireSession(http.HandlerFunc(s.handleProjects))))
	mux.Handle("/api/projects/", s.withCORS(s.requireSession(http.HandlerFunc(s.handleProject))))
	mux.Handle("/api/maintainers/from-ref", s.withCORS(s.requireSession(http.HandlerFunc(s.handleMaintainerFromRef))))
	mux.Handle("/api/maintainers/", s.withCORS(s.requireSession(http.HandlerFunc(s.handleMaintainer))))
	mux.Handle("/api/companies", s.withCORS(s.requireSession(http.HandlerFunc(s.handleCompanies))))
	mux.Handle("/api/", s.withCORS(s.requireSession(http.HandlerFunc(s.handleAPINotImplemented))))

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Printf("web-bff: starting on %s", addr)
	if err := server.ListenAndServe(); err != nil {
		logger.Fatalf("web-bff: server error: %v", err)
	}
}

func openStore(driver, dsn string) (*db.SQLStore, error) {
	gormDB, err := db.OpenGorm(driver, dsn, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	return db.NewSQLStore(gormDB), nil
}

func (s *server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken(32)
	if err != nil {
		http.Error(w, "failed to start login", http.StatusInternalServerError)
		return
	}

	redirectPath := sanitizeRedirect(r.URL.Query().Get(loginRedirectParam))
	s.oauthStates.Set(state, stateEntry{
		Redirect: redirectPath,
		Expires:  time.Now().Add(s.oauthStates.ttl),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     s.stateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   int(s.oauthStates.ttl.Seconds()),
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	url := s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *server) handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing oauth parameters", http.StatusBadRequest)
		return
	}

	stateCookie, err := r.Cookie(s.stateCookie)
	if err != nil || stateCookie.Value != state {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	entry, ok := s.oauthStates.Consume(state)
	if !ok {
		http.Error(w, "oauth state expired", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	token, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "failed to exchange oauth code", http.StatusBadRequest)
		return
	}

	ghUser, err := fetchGitHubUser(ctx, token)
	if err != nil {
		http.Error(w, "failed to fetch github user", http.StatusBadGateway)
		return
	}

	login := strings.ToLower(ghUser.GetLogin())
	role, authorized := s.authorizeLogin(login)
	if !authorized {
		s.logger.Printf("web-bff: unauthorized login attempt from github user %q", login)
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	if err := s.createSession(login, role, w); err != nil {
		http.Error(w, "failed to establish session", http.StatusInternalServerError)
		return
	}

	redirectURL := s.webBaseURL
	if entry.Redirect != "" {
		redirectURL = s.webBaseURL + entry.Redirect
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *server) handleTestLogin(w http.ResponseWriter, r *http.Request) {
	if !s.testMode {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	login := strings.TrimSpace(r.URL.Query().Get("login"))
	if login == "" {
		http.Error(w, "missing login", http.StatusBadRequest)
		return
	}
	role, authorized := s.authorizeLogin(login)
	if !authorized {
		http.Error(w, "unauthorized", http.StatusForbidden)
		return
	}

	if err := s.createSession(login, role, w); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		s.logger.Printf("web-bff: handleTestLogin encode error: %v", err)
	}
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sessionCookie, err := r.Cookie(s.cookieName)
	if err == nil {
		if sess, ok := s.sessions.Delete(sessionCookie.Value); ok {
			duration := time.Since(sess.CreatedAt).Truncate(time.Second)
			s.logger.Printf("web-bff: logout user=%s role=%s session_duration=%s", sess.Login, sess.Role, duration)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    "",
		Path:     "/",
		Domain:   s.cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) createSession(login, role string, w http.ResponseWriter) error {
	sessionID, err := randomToken(48)
	if err != nil {
		return err
	}

	now := time.Now()
	s.sessions.Set(session{
		ID:        sessionID,
		Login:     login,
		Role:      role,
		CreatedAt: now,
		ExpiresAt: now.Add(s.sessionTTL),
	})

	s.logger.Printf("web-bff: login success user=%s role=%s", login, role)

	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    sessionID,
		Path:     "/",
		Domain:   s.cookieDomain,
		MaxAge:   int(s.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	session := sessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	response := map[string]string{
		"login": session.Login,
		"role":  session.Role,
	}
	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("web-bff: handleMe encode error: %v", err)
	}
}

type projectSummary struct {
	ID          uint                `json:"id"`
	Name        string              `json:"name"`
	Maturity    string              `json:"maturity"`
	Maintainers []maintainerSummary `json:"maintainers"`
}

type projectsResponse struct {
	Total    int64            `json:"total"`
	Projects []projectSummary `json:"projects"`
}

type projectIDRow struct {
	ID   uint   `gorm:"column:id"`
	Name string `gorm:"column:name"`
}

type maintainerSummary struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	GitHub string `json:"github"`
}

type projectMaintainerDetail struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	GitHub          string `json:"github"`
	InMaintainerRef bool   `json:"inMaintainerRef"`
}

type serviceSummary struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type maintainerRefStatus struct {
	URL       string     `json:"url,omitempty"`
	Status    string     `json:"status"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
}

type projectDetailResponse struct {
	ID                uint                      `json:"id"`
	Name              string                    `json:"name"`
	Maturity          string                    `json:"maturity"`
	ParentProjectID   *uint                     `json:"parentProjectId,omitempty"`
	MaintainerRef     string                    `json:"maintainerRef,omitempty"`
	RefStatus         maintainerRefStatus       `json:"maintainerRefStatus"`
	MaintainerRefBody string                    `json:"maintainerRefBody,omitempty"`
	RefOnlyGitHub     []string                  `json:"refOnlyGitHub"`
	RefLines          map[string]string         `json:"refLines,omitempty"`
	OnboardingIssue   string                    `json:"onboardingIssue,omitempty"`
	MailingList       string                    `json:"mailingList,omitempty"`
	Maintainers       []projectMaintainerDetail `json:"maintainers"`
	Services          []serviceSummary          `json:"services"`
	CreatedAt         time.Time                 `json:"createdAt"`
	UpdatedAt         time.Time                 `json:"updatedAt"`
	DeletedAt         *time.Time                `json:"deletedAt,omitempty"`
}

func (s *server) handleProjects(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	limit := parseIntParam(r, "limit", 20, 1, 100)
	offset := parseIntParam(r, "offset", 0, 0, 10_000_000)
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "name"
	}
	if sortBy != "name" {
		sortBy = "name"
	}
	direction := strings.ToLower(r.URL.Query().Get("direction"))
	if direction != "desc" {
		direction = "asc"
	}

	maturityFilters := parseCSVParam(r, "maturity")

	base := s.store.DB().Model(&model.Project{})
	if len(maturityFilters) > 0 {
		base = base.Where("projects.maturity IN ?", maturityFilters)
	}
	if query != "" {
		like := "%" + strings.ToLower(query) + "%"
		base = base.
			Joins("LEFT JOIN maintainer_projects mp ON mp.project_id = projects.id").
			Joins("LEFT JOIN maintainers maint ON maint.id = mp.maintainer_id").
			Where(
				"LOWER(projects.name) LIKE ? OR LOWER(maint.name) LIKE ? OR LOWER(maint.git_hub_account) LIKE ?",
				like, like, like,
			)
	}

	var total int64
	if err := base.Distinct("projects.id").Count(&total).Error; err != nil {
		s.logger.Printf("web-bff: handleProjects count error: %v", err)
		http.Error(w, "failed to count projects", http.StatusInternalServerError)
		return
	}

	order := "projects." + sortBy + " " + direction
	var rows []projectIDRow
	if err := base.
		Select("projects.id, projects.name").
		Distinct().
		Order(order).
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		s.logger.Printf("web-bff: handleProjects list ids error: %v", err)
		http.Error(w, "failed to load projects", http.StatusInternalServerError)
		return
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	projects := make([]projectSummary, 0, len(ids))
	if len(ids) > 0 {
		var results []model.Project
		if err := s.store.DB().
			Preload("Maintainers").
			Where("projects.id IN ?", ids).
			Find(&results).Error; err != nil {
			s.logger.Printf("web-bff: handleProjects load projects error: %v", err)
			http.Error(w, "failed to load projects", http.StatusInternalServerError)
			return
		}

		projectByID := make(map[uint]model.Project, len(results))
		for _, project := range results {
			projectByID[project.ID] = project
		}

		for _, id := range ids {
			project, ok := projectByID[id]
			if !ok {
				continue
			}
			maintainers := summarizeMaintainers(project.Maintainers)
			projects = append(projects, projectSummary{
				ID:          project.ID,
				Name:        project.Name,
				Maturity:    string(project.Maturity),
				Maintainers: maintainers,
			})
		}
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(projectsResponse{
		Total:    total,
		Projects: projects,
	}); err != nil {
		s.logger.Printf("web-bff: handleProjects encode error: %v", err)
	}
}

func (s *server) handleProject(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPatch {
		s.handleProjectMaintainerRefUpdate(w, r)
		return
	}
	id, err := parseIDParam(r.URL.Path, "/api/projects/")
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	session := sessionFromContext(r.Context())
	login := "anonymous"
	role := "unknown"
	if session != nil {
		login = session.Login
		role = session.Role
	}
	s.logger.Printf("web-bff: project lookup id=%d path=%s user=%s role=%s", id, r.URL.Path, login, role)

	project, err := s.store.GetProjectByID(id)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			s.logger.Printf("web-bff: project not found id=%d path=%s user=%s role=%s", id, r.URL.Path, login, role)
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		s.logger.Printf("web-bff: failed to load project id=%d path=%s user=%s role=%s err=%v", id, r.URL.Path, login, role, err)
		http.Error(w, "failed to load project", http.StatusInternalServerError)
		return
	}

	refStatus := maintainerRefStatus{Status: "missing"}
	refMatches := make(map[uint]bool)
	refOnlyGitHub := []string{}
	refLines := map[string]string{}
	refURL := strings.TrimSpace(project.MaintainerRef)
	refBody := ""
	if refURL != "" {
		refStatus.URL = refURL
		body, fetchErr := fetchMaintainerRef(r.Context(), refURL)
		if fetchErr != nil {
			refStatus.Status = "error"
		} else {
			refStatus.Status = "fetched"
			checkedAt := time.Now()
			refStatus.CheckedAt = &checkedAt
			refBody = body
			refMatches = buildMaintainerRefMatches(body, project.Maintainers)
			refOnlyGitHub = buildMaintainerRefOnly(body, project.Maintainers)
			refLines = buildMaintainerRefLines(body)
		}
	}
	if role != roleStaff {
		refBody = ""
		refLines = nil
	}
	if refOnlyGitHub == nil {
		refOnlyGitHub = []string{}
	}

	maintainers := summarizeMaintainerDetails(project.Maintainers, refMatches)
	services := make([]serviceSummary, 0, len(project.Services))
	for _, service := range project.Services {
		services = append(services, serviceSummary{
			ID:          service.ID,
			Name:        service.Name,
			Description: service.Description,
		})
	}

	var deletedAt *time.Time
	if project.DeletedAt.Valid {
		ts := project.DeletedAt.Time
		deletedAt = &ts
	}

	response := projectDetailResponse{
		ID:                project.ID,
		Name:              project.Name,
		Maturity:          string(project.Maturity),
		ParentProjectID:   project.ParentProjectID,
		RefStatus:         refStatus,
		MaintainerRefBody: refBody,
		RefOnlyGitHub:     refOnlyGitHub,
		RefLines:          refLines,
		Maintainers:       maintainers,
		Services:          services,
		CreatedAt:         project.CreatedAt,
		UpdatedAt:         project.UpdatedAt,
		DeletedAt:         deletedAt,
	}

	maintainerRef := strings.TrimSpace(project.MaintainerRef)
	if maintainerRef != "" {
		response.MaintainerRef = maintainerRef
	}
	if project.OnboardingIssue != nil {
		onboardingIssue := strings.TrimSpace(*project.OnboardingIssue)
		if onboardingIssue != "" {
			response.OnboardingIssue = onboardingIssue
		}
	}
	if project.MailingList != nil {
		mailingList := strings.TrimSpace(normalizeValue(*project.MailingList, "MML_MISSING"))
		if mailingList != "" {
			response.MailingList = mailingList
		}
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("web-bff: handleProject encode error: %v", err)
	}
}

type projectMaintainerRefUpdateRequest struct {
	MaintainerRef string `json:"maintainerRef"`
}

func (s *server) handleProjectMaintainerRefUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	id, err := parseIDParam(r.URL.Path, "/api/projects/")
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	session := sessionFromContext(r.Context())
	if session == nil || session.Role != roleStaff {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req projectMaintainerRefUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	ref := strings.TrimSpace(req.MaintainerRef)
	if ref != "" && !strings.HasPrefix(ref, "http://") && !strings.HasPrefix(ref, "https://") {
		http.Error(w, "maintainerRef must be a URL", http.StatusBadRequest)
		return
	}
	if err := s.store.UpdateProjectMaintainerRef(id, ref); err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		s.logger.Printf("web-bff: update maintainerRef failed id=%d err=%v", id, err)
		http.Error(w, "failed to update project", http.StatusInternalServerError)
		return
	}
	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		s.logger.Printf("web-bff: handleProject update encode error: %v", err)
	}
}

type maintainerDetailResponse struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	GitHub      string   `json:"github"`
	GitHubEmail string   `json:"githubEmail"`
	Status      string   `json:"status"`
	Company     string   `json:"company,omitempty"`
	Projects    []string `json:"projects"`
}

func (s *server) handleMaintainer(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r.URL.Path, "/api/maintainers/")
	if err != nil {
		http.Error(w, "invalid maintainer id", http.StatusBadRequest)
		return
	}
	session := sessionFromContext(r.Context())
	login := "anonymous"
	role := "unknown"
	if session != nil {
		login = session.Login
		role = session.Role
	}
	s.logger.Printf("web-bff: maintainer lookup id=%d path=%s user=%s role=%s", id, r.URL.Path, login, role)

	var maintainer model.Maintainer
	if err := s.store.DB().
		Preload("Company").
		Preload("Projects").
		First(&maintainer, id).Error; err != nil {
		s.logger.Printf("web-bff: maintainer not found id=%d path=%s user=%s role=%s err=%v", id, r.URL.Path, login, role, err)
		http.Error(w, "maintainer not found", http.StatusNotFound)
		return
	}

	projects := make([]string, 0, len(maintainer.Projects))
	for _, project := range maintainer.Projects {
		projects = append(projects, project.Name)
	}

	response := maintainerDetailResponse{
		ID:          maintainer.ID,
		Name:        maintainer.Name,
		Email:       normalizeValue(maintainer.Email, "EMAIL_MISSING"),
		GitHub:      normalizeValue(maintainer.GitHubAccount, "GITHUB_MISSING"),
		GitHubEmail: normalizeValue(maintainer.GitHubEmail, "GITHUB_MISSING"),
		Status:      string(maintainer.MaintainerStatus),
		Projects:    projects,
	}
	if maintainer.Company.Name != "" {
		response.Company = maintainer.Company.Name
	}

	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("web-bff: handleMaintainer encode error: %v", err)
	}
}

type addMaintainerRequest struct {
	ProjectID    uint   `json:"projectId"`
	Name         string `json:"name"`
	GitHubHandle string `json:"githubHandle"`
	Email        string `json:"email"`
	Company      string `json:"company"`
}

type addMaintainerResponse struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	GitHub  string `json:"github"`
	Email   string `json:"email,omitempty"`
	Company string `json:"company,omitempty"`
}

func (s *server) handleMaintainerFromRef(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	session := sessionFromContext(r.Context())
	if session == nil || session.Role != roleStaff {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req addMaintainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.GitHubHandle = strings.TrimSpace(req.GitHubHandle)
	req.Email = strings.TrimSpace(req.Email)
	req.Company = strings.TrimSpace(req.Company)
	if req.ProjectID == 0 || req.GitHubHandle == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}
	maintainer, err := s.store.CreateMaintainer(req.ProjectID, req.Name, req.Email, req.GitHubHandle, req.Company)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to create maintainer", http.StatusInternalServerError)
		return
	}
	response := addMaintainerResponse{
		ID:     maintainer.ID,
		Name:   maintainer.Name,
		GitHub: normalizeValue(maintainer.GitHubAccount, "GITHUB_MISSING"),
		Email:  normalizeValue(maintainer.Email, "EMAIL_MISSING"),
	}
	if maintainer.Company.Name != "" {
		response.Company = maintainer.Company.Name
	}
	w.Header().Set(headerContentType, contentTypeJSON)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("web-bff: handleCompanies encode error: %v", err)
	}
}

type companyResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type createCompanyRequest struct {
	Name string `json:"name"`
}

func (s *server) handleCompanies(w http.ResponseWriter, r *http.Request) {
	session := sessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		companies, err := s.store.ListCompanies()
		if err != nil {
			http.Error(w, "failed to load companies", http.StatusInternalServerError)
			return
		}
		resp := make([]companyResponse, 0, len(companies))
		for _, company := range companies {
			if strings.TrimSpace(company.Name) == "" {
				continue
			}
			resp = append(resp, companyResponse{
				ID:   company.ID,
				Name: company.Name,
			})
		}
		w.Header().Set(headerContentType, contentTypeJSON)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			s.logger.Printf("web-bff: handleCompanies encode error: %v", err)
		}
	case http.MethodPost:
		if session.Role != roleStaff {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req createCompanyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		company, err := s.store.CreateCompany(req.Name)
		if err != nil {
			if errors.Is(err, db.ErrCompanyExists) {
				http.Error(w, "company already exists", http.StatusConflict)
				return
			}
			http.Error(w, "failed to create company", http.StatusBadRequest)
			return
		}
		w.Header().Set(headerContentType, contentTypeJSON)
		if err := json.NewEncoder(w).Encode(companyResponse{ID: company.ID, Name: company.Name}); err != nil {
			s.logger.Printf("web-bff: handleCompanies encode error: %v", err)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) handleAPINotImplemented(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "api not implemented", http.StatusNotImplemented)
}

func (s *server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.webOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.webOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionCookie, err := r.Cookie(s.cookieName)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		session, ok := s.sessions.Get(sessionCookie.Value)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey{}, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *server) authorizeLogin(login string) (string, bool) {
	if login == "" {
		return "", false
	}

	staff, err := s.store.ListStaffMembers()
	if err != nil {
		s.logger.Printf("web-bff: failed to load staff: %v", err)
		return "", false
	}
	for _, member := range staff {
		if strings.EqualFold(member.GitHubAccount, login) {
			return roleStaff, true
		}
	}

	maintainers, err := s.store.GetMaintainerMapByGitHubAccount()
	if err != nil {
		s.logger.Printf("web-bff: failed to load maintainers: %v", err)
		return "", false
	}
	for ghLogin, maintainer := range maintainers {
		if maintainer.GitHubAccount == "" || maintainer.GitHubAccount == "GITHUB_MISSING" {
			continue
		}
		if strings.EqualFold(ghLogin, login) {
			return roleMaintainer, true
		}
	}

	return "", false
}

func fetchGitHubUser(ctx context.Context, token *oauth2.Token) (*github.User, error) {
	if token == nil {
		return nil, errors.New("missing oauth token")
	}
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(token)))
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, err
	}
	return user, nil
}

type sessionKey struct{}

func sessionFromContext(ctx context.Context) *session {
	if ctx == nil {
		return nil
	}
	if value := ctx.Value(sessionKey{}); value != nil {
		if s, ok := value.(session); ok {
			return &s
		}
	}
	return nil
}

func newSessionStore(logger *log.Logger) *sessionStore {
	return &sessionStore{
		sessions: make(map[string]session),
		logger:   logger,
	}
}

func (s *sessionStore) Set(sess session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
}

func (s *sessionStore) Get(id string) (session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	if !ok {
		return session{}, false
	}
	if time.Now().After(sess.ExpiresAt) {
		if s.logger != nil {
			duration := time.Since(sess.CreatedAt).Truncate(time.Second)
			s.logger.Printf("web-bff: session expired user=%s role=%s session_duration=%s", sess.Login, sess.Role, duration)
		}
		return session{}, false
	}
	return sess, true
}

func (s *sessionStore) Delete(id string) (session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
		return sess, true
	}
	return session{}, false
}

func newStateStore(ttl time.Duration) *stateStore {
	return &stateStore{
		states: make(map[string]stateEntry),
		ttl:    ttl,
	}
}

func (s *stateStore) Set(state string, entry stateEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[state] = entry
}

func (s *stateStore) Consume(state string) (stateEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.states[state]
	if !ok || time.Now().After(entry.Expires) {
		delete(s.states, state)
		return stateEntry{}, false
	}
	delete(s.states, state)
	return entry, true
}

func sanitizeRedirect(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	return ""
}

func originFromBaseURL(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
}

func randomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseIntParam(r *http.Request, key string, fallback, min, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func parseCSVParam(r *http.Request, key string) []string {
	values := r.URL.Query()[key]
	if len(values) == 0 {
		if raw := r.URL.Query().Get(key); raw != "" {
			values = []string{raw}
		}
	}
	var out []string
	for _, value := range values {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item != "" {
				out = append(out, item)
			}
		}
	}
	return out
}

func parseIDParam(path, prefix string) (uint, error) {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return 0, fmt.Errorf("missing id")
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return uint(value), nil
}

func normalizeValue(value, sentinel string) string {
	if value == sentinel {
		return ""
	}
	return value
}

func summarizeMaintainers(maintainers []model.Maintainer) []maintainerSummary {
	seen := make(map[string]struct{})
	result := make([]maintainerSummary, 0, len(maintainers))
	for _, maintainer := range maintainers {
		name := strings.TrimSpace(maintainer.Name)
		github := strings.TrimSpace(maintainer.GitHubAccount)
		if github == "GITHUB_MISSING" {
			github = ""
		}
		key := fmt.Sprintf("%s|%s", name, github)
		if key == "|" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, maintainerSummary{
			ID:     maintainer.ID,
			Name:   name,
			GitHub: github,
		})
	}
	return result
}

func summarizeMaintainerDetails(maintainers []model.Maintainer, refMatches map[uint]bool) []projectMaintainerDetail {
	seen := make(map[string]struct{})
	result := make([]projectMaintainerDetail, 0, len(maintainers))
	for _, maintainer := range maintainers {
		name := strings.TrimSpace(maintainer.Name)
		github := strings.TrimSpace(maintainer.GitHubAccount)
		if github == "GITHUB_MISSING" {
			github = ""
		}
		key := fmt.Sprintf("%s|%s", name, github)
		if key == "|" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, projectMaintainerDetail{
			ID:              maintainer.ID,
			Name:            name,
			GitHub:          github,
			InMaintainerRef: refMatches[maintainer.ID],
		})
	}
	return result
}

func fetchMaintainerRef(ctx context.Context, refURL string) (string, error) {
	rewritten, err := rewriteMaintainerRefURL(refURL)
	if err != nil {
		return "", fmt.Errorf("invalid maintainer ref url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rewritten, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func rewriteMaintainerRefURL(refURL string) (string, error) {
	parsed, err := url.Parse(refURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid maintainer ref url")
	}
	if strings.EqualFold(parsed.Host, "github.com") {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 5 && parts[2] == "blob" {
			org := parts[0]
			repo := parts[1]
			branch := parts[3]
			filePath := strings.Join(parts[4:], "/")
			parsed.Host = "raw.githubusercontent.com"
			parsed.Path = fmt.Sprintf("/%s/%s/%s/%s", org, repo, branch, filePath)
		}
	}
	return parsed.String(), nil
}

func buildMaintainerRefMatches(refBody string, maintainers []model.Maintainer) map[uint]bool {
	matches := make(map[uint]bool)
	if refBody == "" {
		return matches
	}
	for _, maintainer := range maintainers {
		handle := strings.TrimSpace(maintainer.GitHubAccount)
		if handle == "" || handle == "GITHUB_MISSING" {
			continue
		}
		if maintainerRefContains(refBody, handle) {
			matches[maintainer.ID] = true
		}
	}
	return matches
}

func buildMaintainerRefOnly(refBody string, maintainers []model.Maintainer) []string {
	handles := extractGitHubHandles(refBody)
	if len(handles) == 0 {
		return nil
	}
	internal := make(map[string]struct{}, len(maintainers))
	for _, maintainer := range maintainers {
		handle := strings.TrimSpace(maintainer.GitHubAccount)
		if handle == "" || handle == "GITHUB_MISSING" {
			continue
		}
		internal[strings.ToLower(handle)] = struct{}{}
	}
	out := make([]string, 0, len(handles))
	for handle := range handles {
		if _, ok := internal[handle]; !ok {
			out = append(out, handle)
		}
	}
	sort.Strings(out)
	return out
}

func buildMaintainerRefLines(refBody string) map[string]string {
	lines := strings.Split(refBody, "\n")
	result := make(map[string]string)
	if len(lines) == 0 {
		return result
	}
	atRe := regexp.MustCompile(`(?i)(^|[^a-z0-9_-])@([a-z0-9-]{1,39})`)
	urlRe := regexp.MustCompile(`(?i)github\.com/([a-z0-9-]{1,39})`)
	for _, line := range lines {
		for _, match := range atRe.FindAllStringSubmatch(line, -1) {
			if len(match) < 3 {
				continue
			}
			handle := strings.ToLower(match[2])
			if _, ok := result[handle]; !ok {
				result[handle] = strings.TrimSpace(line)
			}
		}
		for _, match := range urlRe.FindAllStringSubmatch(line, -1) {
			if len(match) < 2 {
				continue
			}
			handle := strings.ToLower(match[1])
			if handle == "organizations" || handle == "orgs" || handle == "repos" {
				continue
			}
			if _, ok := result[handle]; !ok {
				result[handle] = strings.TrimSpace(line)
			}
		}
	}
	return result
}

func extractGitHubHandles(refBody string) map[string]struct{} {
	result := make(map[string]struct{})
	if refBody == "" {
		return result
	}
	// Match @username
	atRe := regexp.MustCompile(`(?i)(^|[^a-z0-9_-])@([a-z0-9-]{1,39})`)
	for _, match := range atRe.FindAllStringSubmatch(refBody, -1) {
		if len(match) < 3 {
			continue
		}
		handle := strings.ToLower(match[2])
		result[handle] = struct{}{}
	}
	// Match github.com/username
	urlRe := regexp.MustCompile(`(?i)github\.com/([a-z0-9-]{1,39})`)
	for _, match := range urlRe.FindAllStringSubmatch(refBody, -1) {
		if len(match) < 2 {
			continue
		}
		handle := strings.ToLower(match[1])
		if handle == "organizations" || handle == "orgs" || handle == "repos" {
			continue
		}
		result[handle] = struct{}{}
	}
	return result
}

func maintainerRefContains(refBody, handle string) bool {
	escaped := regexp.QuoteMeta(handle)
	pattern := fmt.Sprintf(`(?i)(^|[^a-z0-9_-])@?%s([^a-z0-9_-]|$)`, escaped)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(refBody)
}
