package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserSettings struct {
	TelegramID int64
	City       string
	Categories []string
}

type TailoredRecord struct {
	ID         int
	PublicSlug string
	MDPath     string
	PDFPath    string
	CreatedAt  time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	s := &Store{pool: pool}
	if err := s.Migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return s, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Migrate(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS user_settings (
    telegram_id BIGINT PRIMARY KEY,
    city TEXT NOT NULL DEFAULT 'Prague',
    categories TEXT[] NOT NULL DEFAULT ARRAY['backend', 'software_engineering']
);

CREATE TABLE IF NOT EXISTS seen_jobs (
    public_slug TEXT PRIMARY KEY,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tailored_history (
    id SERIAL PRIMARY KEY,
    public_slug TEXT NOT NULL,
    md_path TEXT NOT NULL,
    pdf_path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`
	_, err := s.pool.Exec(ctx, schema)
	return err
}

func (s *Store) GetOrCreateUserSettings(ctx context.Context, telegramID int64, defaultCity string, defaultCategories []string) (UserSettings, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_settings (telegram_id, city, categories)
		VALUES ($1, $2, $3)
		ON CONFLICT (telegram_id) DO NOTHING
	`, telegramID, defaultCity, defaultCategories)
	if err != nil {
		return UserSettings{}, fmt.Errorf("upsert default user_settings: %w", err)
	}

	var us UserSettings
	err = s.pool.QueryRow(ctx, `
		SELECT telegram_id, city, categories
		FROM user_settings
		WHERE telegram_id = $1
	`, telegramID).Scan(&us.TelegramID, &us.City, &us.Categories)
	if err != nil {
		return UserSettings{}, fmt.Errorf("select user_settings: %w", err)
	}
	return us, nil
}

func (s *Store) UpdateCity(ctx context.Context, telegramID int64, city string, defaultCategories []string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_settings (telegram_id, city, categories)
		VALUES ($1, $2, $3)
		ON CONFLICT (telegram_id) DO UPDATE SET city = EXCLUDED.city
	`, telegramID, city, defaultCategories)
	return err
}

func (s *Store) UpdateCategories(ctx context.Context, telegramID int64, categories []string, defaultCity string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_settings (telegram_id, city, categories)
		VALUES ($1, $2, $3)
		ON CONFLICT (telegram_id) DO UPDATE SET categories = EXCLUDED.categories
	`, telegramID, defaultCity, categories)
	return err
}

func (s *Store) GetSeenMap(ctx context.Context, slugs []string) (map[string]bool, error) {
	seen := make(map[string]bool, len(slugs))
	if len(slugs) == 0 {
		return seen, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT public_slug
		FROM seen_jobs
		WHERE public_slug = ANY($1)
	`, slugs)
	if err != nil {
		return nil, fmt.Errorf("query seen_jobs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		seen[slug] = true
	}
	return seen, rows.Err()
}

func (s *Store) MarkJobsSeen(ctx context.Context, slugs []string) error {
	if len(slugs) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO seen_jobs (public_slug)
		SELECT UNNEST($1::text[])
		ON CONFLICT (public_slug) DO NOTHING
	`, slugs)
	if err != nil {
		return fmt.Errorf("mark jobs seen: %w", err)
	}
	return nil
}

func (s *Store) SaveTailoredHistory(ctx context.Context, slug, mdPath, pdfPath string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tailored_history (public_slug, md_path, pdf_path)
		VALUES ($1, $2, $3)
	`, slug, mdPath, pdfPath)
	if err != nil {
		return fmt.Errorf("save tailored_history: %w", err)
	}
	return nil
}
