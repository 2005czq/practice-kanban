package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"kanban/internal/codeforces"
	"kanban/internal/dashboard"
	"kanban/ui"
)

const defaultOutputDir = "docs"

type Generator struct {
	config  Config
	service *dashboard.Service
}

func NewGenerator() (*Generator, error) {
	if err := LoadDotEnv(".env"); err != nil {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	client := codeforces.NewClient()
	service := dashboard.NewService(client, dashboard.Options{
		Title:     config.Title,
		Users:     config.Users,
		StartDate: config.StartDate,
		Location:  config.Location,
	})

	return &Generator{
		config:  config,
		service: service,
	}, nil
}

func (g *Generator) Run(ctx context.Context) error {
	payload, err := g.service.Build(ctx)
	if err != nil {
		return fmt.Errorf("build dashboard payload: %w", err)
	}

	if err := os.RemoveAll(defaultOutputDir); err != nil {
		return fmt.Errorf("reset output dir: %w", err)
	}
	if err := os.MkdirAll(defaultOutputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := copyEmbeddedSite(defaultOutputDir); err != nil {
		return fmt.Errorf("copy site assets: %w", err)
	}

	dataDir := filepath.Join(defaultOutputDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	if err := writeJSON(filepath.Join(dataDir, "dashboard.json"), payload); err != nil {
		return fmt.Errorf("write dashboard.json: %w", err)
	}

	if err := writeJSON(g.config.CacheFile, payload); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}

	return nil
}

func copyEmbeddedSite(outputDir string) error {
	root, err := fs.Sub(ui.Files, "web")
	if err != nil {
		return err
	}

	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		return err
	}

	assetsDir := filepath.Join(outputDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		sourceName := entry.Name()
		content, err := fs.ReadFile(root, sourceName)
		if err != nil {
			return err
		}

		destination := filepath.Join(assetsDir, sourceName)
		if sourceName == "index.html" {
			destination = filepath.Join(outputDir, sourceName)
			content = []byte(rewriteIndexAssetPaths(string(content)))
		}

		if err := os.WriteFile(destination, content, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func rewriteIndexAssetPaths(content string) string {
	content = strings.ReplaceAll(content, "/assets/styles.css", "./assets/styles.css")
	content = strings.ReplaceAll(content, "/assets/app.js", "./assets/app.js")
	return content
}

func writeJSON(path string, payload any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
