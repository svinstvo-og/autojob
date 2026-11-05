package tailor

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// RenderMarkdownToPDF converts Markdown content into HTML using goldmark,
// injects it into the HTML/CSS template file on disk, and prints to an A4 PDF via headless Chrome.
func RenderMarkdownToPDF(ctx context.Context, markdownContent, htmlTemplatePath, outputPDFPath string) error {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	var htmlBody bytes.Buffer
	if err := md.Convert([]byte(markdownContent), &htmlBody); err != nil {
		return fmt.Errorf("convert markdown to HTML: %w", err)
	}

	tmplBytes, err := os.ReadFile(htmlTemplatePath)
	if err != nil {
		return fmt.Errorf("read HTML template (%s): %w", htmlTemplatePath, err)
	}

	fullHTML := strings.Replace(string(tmplBytes), "{{.Body}}", htmlBody.String(), 1)

	renderCtx, cancelTimeout := context.WithTimeout(ctx, 45*time.Second)
	defer cancelTimeout()

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(renderCtx, allocOpts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	dataURL := "data:text/html;charset=utf-8;base64," + base64.StdEncoding.EncodeToString([]byte(fullHTML))

	if _, err := chromedp.Run(browserCtx, chromedp.Navigate(dataURL)); err != nil {
		return fmt.Errorf("chromedp navigate: %w", err)
	}

	pdfBuf, err := chromedp.Run(browserCtx, chromedp.PrintToPDF(
		chromedp.PDFPaper(chromedp.PaperA4),
		chromedp.PDFPrintBackground(),
		chromedp.PDFPreferCSSPageSize(),
	))
	if err != nil {
		return fmt.Errorf("chromedp print to PDF: %w", err)
	}

	if err := os.WriteFile(outputPDFPath, pdfBuf, 0o644); err != nil {
		return fmt.Errorf("write PDF file (%s): %w", outputPDFPath, err)
	}
	return nil
}
