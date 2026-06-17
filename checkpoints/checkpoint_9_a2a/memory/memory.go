package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Store manages persistent memories stored as markdown files with YAML frontmatter.
type Store struct {
	dir string
}

// NewStore creates a new memory store. The directory is created if it does not exist.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating memory directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// sanitizeKey converts a key into a safe filename. Only alphanumeric characters,
// hyphens, and underscores are allowed. Everything else is replaced with underscores.
var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitizeKey(key string) string {
	safe := unsafeChars.ReplaceAllString(key, "_")
	// Prevent path traversal.
	safe = strings.Trim(safe, ".")
	if safe == "" {
		safe = "_"
	}
	return safe
}

func (s *Store) path(key string) string {
	return filepath.Join(s.dir, sanitizeKey(key)+".md")
}

// Save stores a memory. If the key already exists, the content and updated timestamp
// are replaced; the original created timestamp is preserved.
func (s *Store) Save(key, content string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	created := now

	// If the file already exists, preserve the original created timestamp.
	existing, err := os.ReadFile(s.path(key))
	if err == nil {
		if ts := extractFrontmatter(string(existing), "created"); ts != "" {
			created = ts
		}
	}

	data := fmt.Sprintf("---\nkey: %q\ncreated: %q\nupdated: %q\n---\n%s\n", key, created, now, content)
	return os.WriteFile(s.path(key), []byte(data), 0o644)
}

// Load retrieves a memory by key. Returns the content (without frontmatter).
func (s *Store) Load(key string) (string, error) {
	data, err := os.ReadFile(s.path(key))
	if err != nil {
		return "", fmt.Errorf("memory %q not found: %w", key, err)
	}
	return stripFrontmatter(string(data)), nil
}

// List returns all stored memory keys.
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("listing memories: %w", err)
	}
	var keys []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		if k := extractFrontmatter(string(data), "key"); k != "" {
			keys = append(keys, k)
		} else {
			// Fallback: use filename without extension.
			keys = append(keys, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return keys, nil
}

// Search performs a case-insensitive substring search across all memories.
// Returns matching entries as "key: content" strings.
func (s *Store) Search(query string) ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("searching memories: %w", err)
	}
	query = strings.ToLower(query)
	var results []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		raw := string(data)
		content := stripFrontmatter(raw)
		key := extractFrontmatter(raw, "key")
		if key == "" {
			key = strings.TrimSuffix(e.Name(), ".md")
		}
		if strings.Contains(strings.ToLower(content), query) || strings.Contains(strings.ToLower(key), query) {
			results = append(results, fmt.Sprintf("%s: %s", key, strings.TrimSpace(content)))
		}
	}
	return results, nil
}

// Dump returns all memories formatted for system prompt injection.
// The total output is capped at maxChars characters.
func (s *Store) Dump(maxChars int) string {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return ""
	}
	var sb strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		raw := string(data)
		content := strings.TrimSpace(stripFrontmatter(raw))
		key := extractFrontmatter(raw, "key")
		if key == "" {
			key = strings.TrimSuffix(e.Name(), ".md")
		}
		line := fmt.Sprintf("- %s: %s\n", key, content)
		if sb.Len()+len(line) > maxChars {
			break
		}
		sb.WriteString(line)
	}
	return sb.String()
}

// extractFrontmatter extracts a value from YAML frontmatter by field name.
func extractFrontmatter(raw, field string) string {
	// Simple parser: look for field between --- delimiters.
	parts := strings.SplitN(raw, "---", 3)
	if len(parts) < 3 {
		return ""
	}
	fm := parts[1]
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		prefix := field + ":"
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			// Strip quotes.
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}

// stripFrontmatter removes YAML frontmatter from a markdown string.
func stripFrontmatter(raw string) string {
	parts := strings.SplitN(raw, "---", 3)
	if len(parts) < 3 {
		return raw
	}
	return strings.TrimSpace(parts[2])
}
