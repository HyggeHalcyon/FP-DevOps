package utils

import "github.com/kennygrant/sanitize"

func SanitizeFilename(filename string) string {
	new := sanitize.HTML(filename)
	new = sanitize.Path(new)
	return new
}
