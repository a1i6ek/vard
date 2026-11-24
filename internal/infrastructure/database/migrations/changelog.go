package migrations

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// ChangelogsDir is where changelogs are written, from the module root.
const ChangelogsDir = "internal/infrastructure/database/migrations/changelogs"

var changelogName = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// NewChangelog writes the next changelog, numbered after the newest
// migration, and says where.
func NewChangelog(dir, name string) (string, error) {
	if !changelogName.MatchString(name) {
		return "", fmt.Errorf("name the changelog in lower case with underscores (add_x), not %q", name)
	}

	all := All()
	version := all[len(all)-1].Version + 1

	words := strings.Split(name, "_")
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}

	var out bytes.Buffer
	err := changelogTemplate.Execute(&out, map[string]any{"Version": version, "Name": strings.Join(words, "")})
	if err != nil {
		return "", err
	}
	source, err := format.Source(out.Bytes())
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, fmt.Sprintf("%05d_%s.go", version, name))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := file.Write(source); err != nil {
		_ = file.Close()
		return "", err
	}
	return path, file.Close()
}

var changelogTemplate = template.Must(template.New("changelog").Parse(`package changelogs

import (
	"context"
	"database/sql"
)

func init() {
	add({{.Version}}, up{{.Name}}, down{{.Name}})
}

// up{{.Name}} …
func up{{.Name}}(_ context.Context, tx *sql.Tx) error {
	return nil
}

// down{{.Name}} undoes it, and says what is lost.
func down{{.Name}}(_ context.Context, tx *sql.Tx) error {
	return nil
}
`))
