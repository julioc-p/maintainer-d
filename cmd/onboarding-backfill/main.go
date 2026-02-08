package main

import (
	"context"
	"log"
	"os"
	"strings"

	"maintainerd/db"
	"maintainerd/model"
	"maintainerd/onboarding"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

const (
	defaultDBPath = "/data/maintainers.db"
)

type issueMatch struct {
	ProjectName string
	URL         string
	Number      int
	Title       string
}

func main() {
	ctx := context.Background()

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

	githubToken := strings.TrimSpace(os.Getenv("GITHUB_API_TOKEN"))
	if githubToken == "" {
		log.Fatal("GITHUB_API_TOKEN is required")
	}

	dbConn, err := openDB(dbDriver, dsn)
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}
	store := db.NewSQLStore(dbConn)

	issues, err := fetchOnboardingIssues(ctx, githubToken)
	if err != nil {
		log.Fatalf("failed to fetch onboarding issues: %v", err)
	}
	issueMap := make(map[string][]issueMatch)
	for _, issue := range issues {
		key := strings.ToLower(strings.TrimSpace(issue.ProjectName))
		if key == "" {
			continue
		}
		issueMap[key] = append(issueMap[key], issue)
	}

	var projects []model.Project
	if err := store.DB().
		Where("onboarding_issue IS NULL OR onboarding_issue = ''").
		Find(&projects).Error; err != nil {
		log.Fatalf("failed to load projects: %v", err)
	}

	updated := 0
	skipped := 0
	for _, project := range projects {
		nameKey := strings.ToLower(strings.TrimSpace(project.Name))
		candidates := issueMap[nameKey]
		if len(candidates) == 0 {
			log.Printf("no onboarding issue found for project %q (id=%d)", project.Name, project.ID)
			skipped++
			continue
		}
		if len(candidates) > 1 {
			log.Printf("multiple onboarding issues for project %q, skipping: %v", project.Name, candidateURLs(candidates))
			skipped++
			continue
		}
		issue := candidates[0]
		if err := store.DB().
			Model(&model.Project{}).
			Where("id = ? AND (onboarding_issue IS NULL OR onboarding_issue = '')", project.ID).
			Update("onboarding_issue", issue.URL).Error; err != nil {
			log.Printf("failed to update project %q (id=%d): %v", project.Name, project.ID, err)
			continue
		}
		updated++
		log.Printf("updated project %q (id=%d) onboarding issue -> %s", project.Name, project.ID, issue.URL)
	}

	log.Printf("backfill complete: updated=%d skipped=%d total=%d", updated, skipped, len(projects))
}

func openDB(driver, dsn string) (*gorm.DB, error) {
	return db.OpenGorm(driver, dsn, &gorm.Config{})
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func fetchOnboardingIssues(ctx context.Context, token string) ([]issueMatch, error) {
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})))
	query := `repo:cncf/sandbox is:issue label:"project onboarding"`
	options := &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 100}}
	issues := make([]issueMatch, 0, 128)
	for {
		result, resp, err := client.Search.Issues(ctx, query, options)
		if err != nil {
			return nil, err
		}
		for _, issue := range result.Issues {
			title := issue.GetTitle()
			projectName, err := onboarding.GetProjectNameFromProjectTitle(title)
			if err != nil {
				log.Printf("skip issue %d: title=%q parse error: %v", issue.GetNumber(), title, err)
				continue
			}
			issues = append(issues, issueMatch{
				ProjectName: projectName,
				URL:         issue.GetHTMLURL(),
				Number:      issue.GetNumber(),
				Title:       title,
			})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		options.Page = resp.NextPage
	}
	log.Printf("fetched onboarding issues=%d", len(issues))
	return issues, nil
}

func candidateURLs(issues []issueMatch) []string {
	urls := make([]string, 0, len(issues))
	for _, issue := range issues {
		urls = append(urls, issue.URL)
	}
	return urls
}
