package app

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid .env entry on line %d", lineNumber)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(stripInlineComment(value))
		if key == "" {
			return fmt.Errorf("empty key on line %d", lineNumber)
		}

		if len(value) >= 2 {
			quote := value[0]
			if (quote == '\'' || quote == '"') && value[len(value)-1] == quote {
				unquoted, err := strconv.Unquote(value)
				if err == nil {
					value = unquoted
				} else {
					value = value[1 : len(value)-1]
				}
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func stripInlineComment(value string) string {
	inSingle := false
	inDouble := false

	for i := 0; i < len(value); i++ {
		current := value[i]
		if current == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if current == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble || i == 0 {
			continue
		}

		if current == '#' && isWhitespace(value[i-1]) {
			return strings.TrimSpace(value[:i])
		}
		if current == '/' && i+1 < len(value) && value[i+1] == '/' && isWhitespace(value[i-1]) {
			return strings.TrimSpace(value[:i])
		}
	}

	return value
}

func isWhitespace(char byte) bool {
	return char == ' ' || char == '\t'
}
