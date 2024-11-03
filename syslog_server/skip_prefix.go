package main

import (
	"regexp"
	"strings"
)

// skipNumericPrefix removes a numeric prefix from the beginning of a string.
// It returns the string without the prefix, or the original string if no prefix is found.
func skipNumericPrefix(line string) string {
	re := regexp.MustCompile(`^<\d+>`) // Matches "<digits>" at the beginning of the string
	return re.ReplaceAllString(line, "")
}

// skipNumericPrefix2 removes a numeric prefix from the beginning of a string.  Handles cases where there might be spaces after the prefix.
// It returns the string without the prefix, or the original string if no prefix is found.
func skipNumericPrefix2(line string) string {
	re := regexp.MustCompile(`^<\d+>\s*`) // Matches "<digits>" at the beginning of the string, allowing for optional spaces after
	return re.ReplaceAllString(line, "")
}


//Example usage
func main() {
	testCases := []string{
		"<123>This is a test message.",
		"<1>Another test message.",
		"<1234567890>Yet another test.",
		"This message has no prefix.",
		"<123>   This message has a prefix and spaces.",
	}

	for _, tc := range testCases {
		fmt.Printf("Original: %s\n", tc)
		fmt.Printf("skipNumericPrefix: %s\n", skipNumericPrefix(tc))
		fmt.Printf("skipNumericPrefix2: %s\n", skipNumericPrefix2(tc))
		fmt.Println("----")
	}
}

