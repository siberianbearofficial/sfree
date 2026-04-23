package manager

import (
	"testing"

	"github.com/example/sfree/api-go/internal/repository"
)

func TestFileSize(t *testing.T) {
	tests := []struct {
		name string
		file repository.File
		want int64
	}{
		{name: "zero chunks", file: repository.File{}, want: 0},
		{
			name: "multi chunk total",
			file: repository.File{Chunks: []repository.FileChunk{
				{Size: 7},
				{Size: 5},
				{Size: 11},
			}},
			want: 23,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FileSize(tt.file); got != tt.want {
				t.Fatalf("FileSize() = %d, want %d", got, tt.want)
			}
		})
	}
}
