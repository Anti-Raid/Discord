package discord

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"
)

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func bytesToBase64Data(data []byte) (string, error) {
	mime, err := getImageMimeType(data)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	base64Encoder := base64.NewEncoder(base64.StdEncoding, &out)

	_, err = base64Encoder.Write(data)
	if err != nil {
		return "", fmt.Errorf("failed to base64 bytes: %w", err)
	}

	defer base64Encoder.Close()

	return "data:" + mime + ";base64," + out.String(), nil
}

func getImageMimeType(data []byte) (string, error) {
	switch {
	case bytes.Equal(data[0:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}):
		return "image/png", nil
	case bytes.Equal(data[0:3], []byte{255, 216, 255}) ||
		bytes.Equal(data[6:10], []byte("JFIF")) ||
		bytes.Equal(data[6:10], []byte("Exif")):
		return "image/jpeg", nil
	case bytes.Equal(data[0:6], []byte{71, 73, 70, 56, 55, 97}) ||
		bytes.Equal(data[0:6], []byte{71, 73, 70, 56, 57, 97}):
		return "image/gif", nil
	case bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp", nil
	default:
		return "", ErrUnsupportedImageType
	}
}

func multipartBodyWithJSON(data interface{}, files []File) (contentType string, body []byte, err error) {
	requestBody := &bytes.Buffer{}
	writer := multipart.NewWriter(requestBody)

	payload, err := json.Marshal(data)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var part io.Writer

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="payload_json"`)
	header.Set("Content-Type", "application/json")

	part, err = writer.CreatePart(header)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create part: %w", err)
	}

	_, err = part.Write(payload)
	if err != nil {
		return "", nil, fmt.Errorf("failed to write payload: %w", err)
	}

	for fileIndex, file := range files {
		header := make(textproto.MIMEHeader)
		header.Set(
			"Content-Disposition",
			fmt.Sprintf(
				`form-data; name="file%d"; filename="%s"`,
				fileIndex, quoteEscaper.Replace(file.Name),
			),
		)

		fileContentType := file.ContentType
		if fileContentType == "" {
			fileContentType = "application/octet-stream"
		}

		header.Set("Content-Type", fileContentType)

		part, err = writer.CreatePart(header)
		if err != nil {
			return "", nil, fmt.Errorf("failed to create part: %w", err)
		}

		_, err = io.Copy(part, file.Reader)
		if err != nil {
			return "", nil, fmt.Errorf("failed to copy file: %w", err)
		}
	}

	err = writer.Close()
	if err != nil {
		return "", nil, fmt.Errorf("failed to close writer")
	}

	return writer.FormDataContentType(), requestBody.Bytes(), nil
}
