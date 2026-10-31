package compile

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// PDFTools reads metadata/text out of produced PDFs (poppler-utils).
// The real implementation is PopplerTools; tests use fakes.
type PDFTools interface {
	PageCount(pdfPath string) (int, error)
	ExtractText(pdfPath string) (string, error)
}

// pdfToolTimeout bounds pdfinfo/pdftotext runs.
const pdfToolTimeout = 10 * time.Second

// maxExtractText caps pdftotext output.
const maxExtractText = 1 << 20

// PopplerTools shells out to poppler-utils. Runs outside the sandbox on PDFs
// we produced ourselves.
type PopplerTools struct {
	PdfinfoPath   string // default "pdfinfo"
	PdftotextPath string // default "pdftotext"
}

func (p PopplerTools) PageCount(pdfPath string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), pdfToolTimeout)
	defer cancel()
	bin := p.PdfinfoPath
	if bin == "" {
		bin = "pdfinfo"
	}
	out, err := exec.CommandContext(ctx, bin, pdfPath).Output()
	if err != nil {
		return 0, fmt.Errorf("pdfinfo failed: %w", err)
	}
	return ParsePages(out)
}

func (p PopplerTools) ExtractText(pdfPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), pdfToolTimeout)
	defer cancel()
	bin := p.PdftotextPath
	if bin == "" {
		bin = "pdftotext"
	}
	cmd := exec.CommandContext(ctx, bin, "-nopgbrk", pdfPath, "-")
	var out cappedBuffer
	out.max = maxExtractText
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext failed: %w", err)
	}
	return out.buf.String(), nil
}

var pagesRe = regexp.MustCompile(`(?m)^Pages:\s+(\d+)`)

// ParsePages reads the Pages field from pdfinfo output.
func ParsePages(pdfinfoOutput []byte) (int, error) {
	m := pagesRe.FindSubmatch(pdfinfoOutput)
	if m == nil {
		return 0, fmt.Errorf("pdfinfo output has no Pages field")
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("pdfinfo output has invalid Pages field")
	}
	return n, nil
}
