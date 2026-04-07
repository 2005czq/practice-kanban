package app

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultCacheFile = "data/cache.json"
)

type Config struct {
	Title     string
	Users     []string
	StartDate time.Time
	CacheFile string
	Location  *time.Location
}

func LoadConfig() (Config, error) {
	tzName := strings.TrimSpace(getEnv("TZ", "Asia/Shanghai"))
	location, err := time.LoadLocation(tzName)
	if err != nil {
		return Config{}, fmt.Errorf("load TZ %q: %w", tzName, err)
	}

	title := strings.TrimSpace(os.Getenv("TITLE"))
	if title == "" {
		return Config{}, fmt.Errorf("TITLE is required")
	}

	usersRaw := strings.TrimSpace(os.Getenv("USERS"))
	if usersRaw == "" {
		return Config{}, fmt.Errorf("USERS is required")
	}

	var users []string
	seen := make(map[string]struct{})
	for _, part := range strings.Split(usersRaw, ",") {
		handle := strings.TrimSpace(part)
		if handle == "" {
			continue
		}
		if _, ok := seen[strings.ToLower(handle)]; ok {
			continue
		}
		seen[strings.ToLower(handle)] = struct{}{}
		users = append(users, handle)
	}
	if len(users) == 0 {
		return Config{}, fmt.Errorf("USERS must contain at least one handle")
	}

	timeRaw := strings.TrimSpace(os.Getenv("TIME"))
	if timeRaw == "" {
		return Config{}, fmt.Errorf("TIME is required")
	}
	startDate, err := time.ParseInLocation("20060102", timeRaw, location)
	if err != nil {
		return Config{}, fmt.Errorf("parse TIME: %w", err)
	}

	cacheFile := strings.TrimSpace(getEnv("CACHE_FILE", defaultCacheFile))

	return Config{
		Title:     title,
		Users:     users,
		StartDate: startDate,
		CacheFile: cacheFile,
		Location:  location,
	}, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
