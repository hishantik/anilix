package miruro

import (
	"fmt"
	"strings"
	"unicode"
)

func WatchURL(anilistID int, title string) string {
	var slug strings.Builder
	separator := false
	for _, r := range strings.ToLower(title) {
		if r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			if separator && slug.Len() > 0 {
				slug.WriteByte('-')
			}
			separator = false
			slug.WriteRune(r)
		} else if slug.Len() > 0 {
			separator = true
		}
	}
	value := strings.Trim(slug.String(), "-")
	if value == "" {
		value = "anime"
	}
	return fmt.Sprintf("https://www.miruro.tv/watch/%d/%s", anilistID, value)
}
