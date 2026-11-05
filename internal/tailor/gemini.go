package tailor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"autojob/internal/freehire"
)

type Config struct {
	GeminiBin          string
	GeminiModel        string
	BaseCVPath         string
	PromptTemplatePath string
	HTMLTemplatePath   string
	OutputDir          string
}

type Result struct {
	MDPath   string
	PDFPath  string
	Markdown string
}

type Service struct {
	cfg Config
}

func NewService(cfg Config) *Service {
	return &Service{cfg: cfg}
}

type promptData struct {
	JobTitle       string
	Company        string
	Location       string
	Skills         string
	JobDescription string
	BaseCV         string
}

// GenerateTailoredCV invokes the local Gemini CLI to produce a tailored Markdown CV,
// writes the .md file, and renders the .pdf file via headless Chromium (chromedp).
func (s *Service) GenerateTailoredCV(ctx context.Context, job freehire.Job) (*Result, error) {
	baseCVBytes, err := os.ReadFile(s.cfg.BaseCVPath)
	if err != nil {
		return nil, fmt.Errorf("read base CV (%s): %w", s.cfg.BaseCVPath, err)
	}

	promptTmplBytes, err := os.ReadFile(s.cfg.PromptTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("read prompt template (%s): %w", s.cfg.PromptTemplatePath, err)
	}

	tmpl, err := template.New("prompt").Parse(string(promptTmplBytes))
	if err != nil {
		return nil, fmt.Errorf("parse prompt template: %w", err)
	}

	var renderedPrompt bytes.Buffer
	err = tmpl.Execute(&renderedPrompt, promptData{
		JobTitle:       job.Title,
		Company:        job.Company,
		Location:       job.Location,
		Skills:         strings.Join(job.Skills, ", "),
		JobDescription: job.Description,
		BaseCV:         string(baseCVBytes),
	})
	if err != nil {
		return nil, fmt.Errorf("render prompt template: %w", err)
	}

	markdown, err := s.runGeminiCLI(ctx, renderedPrompt.String())
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(s.cfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	safeSlug := sanitizeSlug(job.PublicSlug)
	timestamp := time.Now().Format("20060102_150405")
	baseName := fmt.Sprintf("cv_%s_%s", safeSlug, timestamp)

	mdPath := filepath.Join(s.cfg.OutputDir, baseName+".md")
	pdfPath := filepath.Join(s.cfg.OutputDir, baseName+".pdf")

	if err := os.WriteFile(mdPath, []byte(markdown), 0o644); err != nil {
		return nil, fmt.Errorf("write tailored markdown: %w", err)
	}

	if err := RenderMarkdownToPDF(ctx, markdown, s.cfg.HTMLTemplatePath, pdfPath); err != nil {
		return nil, fmt.Errorf("render PDF: %w", err)
	}

	return &Result{
		MDPath:   mdPath,
		PDFPath:  pdfPath,
		Markdown: markdown,
	}, nil
}

func (s *Service) runGeminiCLI(ctx context.Context, prompt string) (string, error) {
	args := []string{
		"-p", "Follow the instructions provided on stdin and output ONLY the tailored CV in Markdown format without code fences.",
	}
	if s.cfg.GeminiModel != "" {
		args = append([]string{"-m", s.cfg.GeminiModel}, args...)
	}

	cmd := exec.CommandContext(ctx, s.cfg.GeminiBin, args...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("gemini CLI failed (%w): %s", err, errMsg)
	}

	cleaned := StripMarkdownCodeFence(stdout.String())
	if strings.TrimSpace(cleaned) == "" {
		return "", fmt.Errorf("gemini CLI returned empty output (stderr: %s)", strings.TrimSpace(stderr.String()))
	}
	return cleaned, nil
}

// StripMarkdownCodeFence strips outer ```markdown ... ``` or ``` ... ``` fences
// and any conversational preamble before the first Markdown heading if present.
func StripMarkdownCodeFence(raw string) string {
	s := strings.TrimSpace(raw)

	// If the output contains a fenced code block (possibly after a short preamble)
	if startFence := strings.Index(s, "```"); startFence != -1 {
		afterFence := s[startFence+3:]
		if nl := strings.IndexByte(afterFence, '\n'); nl != -1 {
			afterFence = afterFence[nl+1:]
		}
		if endFence := strings.LastIndex(afterFence, "```"); endFence != -1 {
			s = strings.TrimSpace(afterFence[:endFence])
		} else {
			s = strings.TrimSpace(afterFence)
		}
	}

	// If Gemini included a brief intro before `# Name`, trim up to the first `# ` heading
	if !strings.HasPrefix(s, "#") {
		if idx := strings.Index(s, "\n# "); idx != -1 {
			s = strings.TrimSpace(s[idx+1:])
		}
	}

	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s) + "\n"
}

var nonAlnumRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitizeSlug(slug string) string {
	s := nonAlnumRegex.ReplaceAllString(slug, "_")
	s = strings.Trim(s, "_")
	if len(s) > 60 {
		s = s[:60]
	}
	if s == "" {
		return "job"
	}
	return s
}
