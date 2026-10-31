package compile

import "testing"

const samplePdfinfo = `Title:
Creator:        LaTeX
Producer:       pdfTeX-1.40.22
Pages:          1
Encrypted:      no
Page size:      612 x 792 pts (letter)
`

func TestParsePages(t *testing.T) {
	n, err := ParsePages([]byte(samplePdfinfo))
	if err != nil || n != 1 {
		t.Fatalf("ParsePages = %d, %v; want 1, nil", n, err)
	}
	if _, err := ParsePages([]byte("no pages here")); err == nil {
		t.Error("missing Pages field should error")
	}
	if _, err := ParsePages([]byte("Pages:          0")); err == nil {
		t.Error("zero pages should error")
	}
}
