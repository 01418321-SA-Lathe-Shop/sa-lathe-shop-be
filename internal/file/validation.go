package file

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/binary"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
)

var allowed = map[string]string{
	".pdf": "application/pdf", ".doc": "application/msword", ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls": "application/vnd.ms-excel", ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".txt": "text/plain", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
}

func ValidateInput(in Input) error {
	if !utf8.ValidString(in.Name) || utf8.RuneCountInString(in.Name) < 1 || utf8.RuneCountInString(in.Name) > 200 || strings.ContainsAny(in.Name, "/\\\x00\r\n") || in.Size < 1 || in.Size > MaxBytes {
		return req.ErrInvalidInput
	}
	if mime, ok := allowed[strings.ToLower(filepath.Ext(in.Name))]; !ok || in.ContentType != mime {
		return req.ErrInvalidInput
	}
	return nil
}

// Content validation complements extension/MIME checks; it is not malware scanning.
// Files are always downloaded as attachments, never rendered as active content.
func ValidateContent(r io.ReaderAt, size int64, in Input) error {
	if err := ValidateInput(in); err != nil {
		return err
	}
	if size != in.Size {
		return req.ErrInvalidInput
	}
	header := make([]byte, 512)
	n, _ := r.ReadAt(header, 0)
	header = header[:n]
	switch strings.ToLower(filepath.Ext(in.Name)) {
	case ".pdf":
		if !bytes.HasPrefix(header, []byte("%PDF-")) {
			return req.ErrInvalidInput
		}
	case ".jpg", ".jpeg", ".png":
		cfg, format, err := image.DecodeConfig(io.NewSectionReader(r, 0, size))
		if err != nil || int64(cfg.Width)*int64(cfg.Height) > 100000000 || cfg.Width <= 0 || cfg.Height <= 0 {
			return req.ErrInvalidInput
		}
		if (in.ContentType == "image/png" && format != "png") || (in.ContentType == "image/jpeg" && format != "jpeg") {
			return req.ErrInvalidInput
		}
	case ".doc", ".xls":
		if len(header) < 8 || binary.LittleEndian.Uint64(header[:8]) != 0xe11ab1a1e011cfd0 {
			return req.ErrInvalidInput
		}
	case ".docx", ".xlsx":
		z, err := zip.NewReader(r, size)
		if err != nil || len(z.File) > 10000 {
			return req.ErrInvalidInput
		}
		wanted := "word/document.xml"
		if strings.HasSuffix(strings.ToLower(in.Name), ".xlsx") {
			wanted = "xl/workbook.xml"
		}
		found, types := false, false
		for _, f := range z.File {
			if strings.Contains(strings.ToLower(f.Name), "vbaproject") {
				return req.ErrInvalidInput
			}
			if f.Name == wanted {
				found = true
			}
			if f.Name == "[Content_Types].xml" {
				types = true
			}
		}
		if !found || !types {
			return req.ErrInvalidInput
		}
	case ".txt":
		reader := bufio.NewReaderSize(io.NewSectionReader(r, 0, size), 64*1024)
		for {
			value, width, err := reader.ReadRune()
			if err == io.EOF {
				break
			}
			if err != nil || value == 0 || (value == utf8.RuneError && width == 1) {
				return req.ErrInvalidInput
			}
		}
	default:
		return req.ErrInvalidInput
	}
	return nil
}
