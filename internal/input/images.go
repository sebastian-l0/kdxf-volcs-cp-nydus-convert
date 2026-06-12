package input

import (
	"bufio"
	"os"
	"strings"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

type ImageLine struct {
	LineNumber int
	Raw        string
}

func LoadImageLines(path string) ([]ImageLine, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperrors.Wrap(apperrors.CodeImageFileNotFound, "image list file not found", err)
		}
		return nil, apperrors.Wrap(apperrors.CodeImageFileNotFound, "failed to open image list file", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lines []ImageLine
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		lines = append(lines, ImageLine{LineNumber: lineNo, Raw: text})
	}
	if err := scanner.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CodeImageFileNotFound, "failed to read image list file", err)
	}
	return lines, nil
}
