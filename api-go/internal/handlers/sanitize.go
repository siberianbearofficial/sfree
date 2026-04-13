package handlers

import "strings"

// contentDispositionReplacer strips characters that could inject into
// a Content-Disposition header value: double-quotes and CRLF.
var contentDispositionReplacer = strings.NewReplacer(`"`, "", "\r", "", "\n", "")

// sanitizeFilename returns a filename safe for use in a Content-Disposition
// header's filename="..." parameter (RFC 6266).
func sanitizeFilename(name string) string {
	return contentDispositionReplacer.Replace(name)
}
