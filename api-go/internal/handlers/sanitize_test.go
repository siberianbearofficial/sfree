package handlers

import "testing"

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"clean name", "report.pdf", "report.pdf"},
		{"double quotes", `file"name.txt`, "filename.txt"},
		{"CRLF injection", "file\r\nInjected-Header: val", "fileInjected-Header: val"},
		{"CR only", "file\rname.txt", "filename.txt"},
		{"LF only", "file\nname.txt", "filename.txt"},
		{"combined", "a\"\r\nb", "ab"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
