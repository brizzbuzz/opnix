package config

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const placeholderPrefix = "<OPNIX:"

// Placeholder returns the reserved marker for a named secret.
func Placeholder(name string) string {
	sum := sha256.Sum256([]byte(name))
	return fmt.Sprintf("%s%x>", placeholderPrefix, sum)
}

// RenderTemplate substitutes known markers in one pass without interpreting values.
// Errors deliberately exclude both template content and secret values.
func RenderTemplate(content string, values map[string]string) (string, error) {
	markers := make(map[string]string, len(values))
	for name, value := range values {
		markers[Placeholder(name)] = value
	}
	var result strings.Builder
	for {
		start := strings.Index(content, placeholderPrefix)
		if start < 0 {
			result.WriteString(content)
			return result.String(), nil
		}
		result.WriteString(content[:start])
		content = content[start:]
		end := strings.IndexByte(content, '>')
		if end < 0 {
			return "", fmt.Errorf("malformed or unknown reserved template marker")
		}
		marker := content[:end+1]
		value, ok := markers[marker]
		if !ok {
			return "", fmt.Errorf("malformed or unknown reserved template marker")
		}
		result.WriteString(value)
		content = content[end+1:]
	}
}
