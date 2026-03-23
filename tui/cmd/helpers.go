package cmd

import "regexp"

// versePattern returns the compiled regex for [N] verse markers.
// Kept here so read.go and speak.go can share it.
func versePattern() *regexp.Regexp {
	return regexp.MustCompile(`\[(\d+)\]`)
}
