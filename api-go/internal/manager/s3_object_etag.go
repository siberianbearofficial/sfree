package manager

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
)

func ObjectETag(file repository.File) string {
	if file.ETag != "" {
		return file.ETag
	}
	return legacyObjectETag(file)
}

func newObjectETag(file repository.File) string {
	if etag, ok := checksumObjectETag(file.Chunks); ok {
		return etag
	}
	return legacyObjectETag(file)
}

func copyObjectETag(source repository.File) string {
	if source.ETag != "" {
		return source.ETag
	}
	if etag, ok := checksumObjectETag(source.Chunks); ok {
		return etag
	}
	return legacyObjectETag(source)
}

func checksumObjectETag(chunks []repository.FileChunk) (string, bool) {
	h := sha256.New()
	for _, chunk := range chunks {
		if chunk.Checksum == "" {
			return "", false
		}
		_, _ = h.Write([]byte(chunk.Checksum))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(strconv.FormatInt(chunk.Size, 10)))
		_, _ = h.Write([]byte(";"))
	}
	return "\"" + hex.EncodeToString(h.Sum(nil)) + "\"", true
}

func legacyObjectETag(file repository.File) string {
	h := sha256.New()
	_, _ = h.Write([]byte(file.Name))
	_, _ = h.Write([]byte(file.CreatedAt.UTC().Format(time.RFC3339Nano)))
	for _, chunk := range file.Chunks {
		_, _ = h.Write([]byte(chunk.SourceID.Hex()))
		_, _ = h.Write([]byte(chunk.Name))
		_, _ = h.Write([]byte(strconv.Itoa(chunk.Order)))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(strconv.FormatInt(chunk.Size, 10)))
	}
	return "\"" + hex.EncodeToString(h.Sum(nil)) + "\""
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}
