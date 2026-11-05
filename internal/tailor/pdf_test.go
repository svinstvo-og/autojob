package tailor

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripMarkdownCodeFence(t *testing.T) {
	input := "Here is your tailored CV:\n\n```markdown\n# Maksym Koval\n**Backend Engineer**\n```\n"
	got := StripMarkdownCodeFence(input)
	if !strings.HasPrefix(got, "# Maksym Koval") {
		t.Errorf("expected heading at start, got: %q", got)
	}
	if strings.Contains(got, "```") {
		t.Errorf("expected code fences removed, got: %q", got)
	}
}

func TestRenderMarkdownToPDF(t *testing.T) {
	tmpDir := t.TempDir()
	outPDF := filepath.Join(tmpDir, "test_cv.pdf")

	mdBytes, err := os.ReadFile("../../assets/base_cv.md")
	if err != nil {
		t.Fatalf("read base_cv.md: %v", err)
	}

	err = RenderMarkdownToPDF(context.Background(), string(mdBytes), "../../assets/cv_template.html", outPDF)
	if err != nil {
		t.Fatalf("RenderMarkdownToPDF failed: %v", err)
	}

	pdfData, err := os.ReadFile(outPDF)
	if err != nil {
		t.Fatalf("read generated PDF: %v", err)
	}
	if !bytes.HasPrefix(pdfData, []byte("%PDF-")) {
		t.Fatalf("generated file is not a valid PDF (size=%d)", len(pdfData))
	}
}
