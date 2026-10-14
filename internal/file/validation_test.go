package file

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"

	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
)

func TestFilePolicy(t *testing.T) {
	for _, in := range []Input{{Name: "a.exe", ContentType: "application/octet-stream", Size: 1}, {Name: "../a.pdf", ContentType: "application/pdf", Size: 1}, {Name: "a.svg", ContentType: "image/svg+xml", Size: 1}, {Name: "a.pdf", ContentType: "image/png", Size: 1}, {Name: "a.pdf", ContentType: "application/pdf", Size: MaxBytes + 1}, {Name: "a.txt", ContentType: "text/plain", Size: 0}} {
		if !errors.Is(ValidateInput(in), req.ErrInvalidInput) {
			t.Fatalf("accepted %s", in.Name)
		}
	}
}
func TestContentMismatch(t *testing.T) {
	data := []byte("not really a PDF")
	in := Input{Name: "drawing.pdf", ContentType: "application/pdf", Size: int64(len(data))}
	if !errors.Is(ValidateContent(bytes.NewReader(data), int64(len(data)), in), req.ErrInvalidInput) {
		t.Fatal("extension-only validation")
	}
	data = []byte("รายละเอียดงาน ไทย\n")
	in = Input{Name: "notes.txt", ContentType: "text/plain", Size: int64(len(data))}
	if err := ValidateContent(bytes.NewReader(data), int64(len(data)), in); err != nil {
		t.Fatal(err)
	}
	if err := ValidateContent(bytes.NewReader(data), int64(len(data))-1, in); !errors.Is(err, req.ErrInvalidInput) {
		t.Fatal("size mismatch accepted")
	}
}
func TestOfficeContainer(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, name := range []string{"[Content_Types].xml", "word/document.xml"} {
		w, _ := z.Create(name)
		w.Write([]byte("<xml/>"))
	}
	z.Close()
	in := Input{Name: "file.docx", ContentType: allowed[".docx"], Size: int64(buf.Len())}
	if err := ValidateContent(bytes.NewReader(buf.Bytes()), in.Size, in); err != nil {
		t.Fatal(err)
	}
	in.Name = "file.xlsx"
	in.ContentType = allowed[".xlsx"]
	if !errors.Is(ValidateContent(bytes.NewReader(buf.Bytes()), in.Size, in), req.ErrInvalidInput) {
		t.Fatal("wrong office format accepted")
	}
}
