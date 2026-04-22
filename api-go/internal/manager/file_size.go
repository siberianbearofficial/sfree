package manager

import "github.com/example/sfree/api-go/internal/repository"

func FileSize(file repository.File) int64 {
	var total int64
	for _, chunk := range file.Chunks {
		total += chunk.Size
	}
	return total
}
