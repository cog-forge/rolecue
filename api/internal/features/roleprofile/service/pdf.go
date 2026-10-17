package service

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
)

const MaxPDFBytes int64 = 5 << 20

type PDFParser interface {
	Parse(context.Context, []byte) (string, error)
}
type PopplerParser struct{}

// limitWriter bounds subprocess output while continuing to drain the pipe.
type limitWriter struct {
	bytes.Buffer
	overflow bool
}

func (w *limitWriter) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (1 << 20) - w.Len()
	if n > remaining {
		w.overflow = true
		p = p[:remaining]
	}
	_, _ = w.Buffer.Write(p)
	return n, nil
}

func (PopplerParser) Parse(ctx context.Context, data []byte) (string, error) {
	if len(data) == 0 || int64(len(data)) > MaxPDFBytes || !bytes.HasPrefix(data, []byte("%PDF-")) {
		return "", apperror.New(apperror.CodeInvalidPDF, "Choose a valid PDF up to 5 MB.")
	}
	tool, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", apperror.New(apperror.CodePDFUnavailable, "PDF reading is unavailable. Paste your JD text to continue.")
	}
	info, err := exec.LookPath("pdfinfo")
	if err != nil {
		return "", apperror.New(apperror.CodePDFUnavailable, "PDF reading is unavailable. Paste your JD text to continue.")
	}
	dir, err := os.MkdirTemp("", "rolecue-pdf-")
	if err != nil {
		return "", apperror.New(apperror.CodeInternal, "Could not read this PDF. Please retry.")
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path := dir + "/source.pdf"
	if err = os.WriteFile(path, data, 0600); err != nil {
		return "", apperror.New(apperror.CodeInternal, "Could not read this PDF. Please retry.")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	metadata := exec.CommandContext(ctx, info, path)
	metadata.Env = append(os.Environ(), "LC_ALL=C")
	var details limitWriter
	metadata.Stdout = &details
	if err = metadata.Run(); err != nil || details.overflow {
		return "", apperror.New(apperror.CodeInvalidPDF, "This PDF could not be read. Use an unencrypted PDF with selectable text, or paste your JD.")
	}
	for _, line := range strings.Split(details.String(), "\n") {
		if strings.HasPrefix(line, "Encrypted:") && strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(line, "Encrypted:")), "yes") {
			return "", apperror.New(apperror.CodeInvalidPDF, "Encrypted PDFs are not supported. Use an unencrypted copy or paste your JD text.")
		}
	}
	cmd := exec.CommandContext(ctx, tool, "-layout", "-enc", "UTF-8", "-nopgbrk", path, "-")
	var output limitWriter
	cmd.Stdout = &output
	// Discard stderr: it can contain document text and internal paths.
	if err = cmd.Run(); err != nil {
		return "", apperror.New(apperror.CodeInvalidPDF, "This PDF could not be read. Use an unencrypted PDF with selectable text, or paste your JD.")
	}
	if output.overflow {
		return "", apperror.New(apperror.CodeJDTooLong, "The PDF contains too much text. Use a JD of 100–20,000 characters.")
	}
	if len(bytes.TrimSpace(output.Bytes())) == 0 {
		return "", apperror.New(apperror.CodeInvalidPDF, "This PDF has no readable text. Scanned PDFs need OCR; paste your JD text instead.")
	}
	return NormalizeInput(output.String())
}
