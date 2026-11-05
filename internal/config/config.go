package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Hardcoded fallback defaults — you can fill in your credentials here directly
// or provide them via .env / environment variables.
const (
	DefaultTelegramBotToken   = ""
	DefaultAllowedTelegramIDs = "" // Comma-separated Telegram user IDs, e.g. "123456789"
	DefaultDatabaseURL        = "postgres://postgres:postgres@localhost:5432/autojob?sslmode=disable"
	DefaultFreehireBaseURL    = "https://freehire.me/api/v1"
	DefaultFreehireAPIKey     = ""
	DefaultCity               = "Prague"
	DefaultCategories         = "backend,software_engineering"
	DefaultLookbackDays       = 7
	DefaultGeminiBin          = "gemini"
	DefaultGeminiModel        = ""
	DefaultBaseCVPath         = "./assets/base_cv.md"
	DefaultPromptTemplatePath = "./assets/prompt.tmpl"
	DefaultHTMLTemplatePath   = "./assets/cv_template.html"
	DefaultOutputDir          = "./output"
)

type Config struct {
	TelegramBotToken   string
	AllowedTelegramIDs map[int64]bool
	DatabaseURL        string
	FreehireBaseURL    string
	FreehireAPIKey     string
	DefaultCity        string
	DefaultCategories  []string
	DefaultLookback    int
	GeminiBin          string
	GeminiModel        string
	BaseCVPath         string
	PromptTemplatePath string
	HTMLTemplatePath   string
	OutputDir          string
}

func Load() (*Config, error) {
	loadDotEnv(".env")

	cfg := &Config{
		TelegramBotToken:   getEnv("TELEGRAM_BOT_TOKEN", DefaultTelegramBotToken),
		DatabaseURL:        getEnv("DATABASE_URL", DefaultDatabaseURL),
		FreehireBaseURL:    strings.TrimRight(getEnv("FREEHIRE_BASE_URL", DefaultFreehireBaseURL), "/"),
		FreehireAPIKey:     getEnv("FREEHIRE_API_KEY", DefaultFreehireAPIKey),
		DefaultCity:        getEnv("DEFAULT_CITY", DefaultCity),
		DefaultCategories:  parseCSV(getEnv("DEFAULT_CATEGORIES", DefaultCategories)),
		DefaultLookback:    getEnvInt("DEFAULT_LOOKBACK_DAYS", DefaultLookbackDays),
		GeminiBin:          getEnv("GEMINI_BIN", DefaultGeminiBin),
		GeminiModel:        getEnv("GEMINI_MODEL", DefaultGeminiModel),
		BaseCVPath:         getEnv("BASE_CV_PATH", DefaultBaseCVPath),
		PromptTemplatePath: getEnv("PROMPT_TEMPLATE_PATH", DefaultPromptTemplatePath),
		HTMLTemplatePath:   getEnv("HTML_TEMPLATE_PATH", DefaultHTMLTemplatePath),
		OutputDir:          getEnv("OUTPUT_DIR", DefaultOutputDir),
		AllowedTelegramIDs: make(map[int64]bool),
	}

	rawIDs := getEnv("ALLOWED_TELEGRAM_IDS", DefaultAllowedTelegramIDs)
	for _, part := range parseCSV(rawIDs) {
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid telegram ID %q in ALLOWED_TELEGRAM_IDS: %w", part, err)
		}
		cfg.AllowedTelegramIDs[id] = true
	}

	if cfg.TelegramBotToken == "" || cfg.TelegramBotToken == "your_bot_token_here" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required (set in .env or internal/config/config.go)")
	}
	if len(cfg.AllowedTelegramIDs) == 0 {
		return nil, fmt.Errorf("ALLOWED_TELEGRAM_IDS is required (set at least one Telegram user ID in .env or internal/config/config.go)")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := getEnv(key, "")
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func parseCSV(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
