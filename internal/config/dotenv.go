package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

func LoadDotenv() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(dir, ".env")
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			if err := parseDotenv(path); err != nil {
				return path, err
			}
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", nil
}

func parseDotenv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		eq := strings.IndexByte(line, '=')
		if eq == -1 {
			continue
		}

		key := strings.TrimSpace(line[:eq])
		if key == "" {
			continue
		}

		value := strings.TrimSpace(line[eq+1:])
		if len(value) >= 2 {
			quote := value[0]
			if quote == '"' || quote == '\'' {
				if value[len(value)-1] == quote {
					value = value[1 : len(value)-1]
				}
			}
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}
