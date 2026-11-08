package storage

import (
    "errors"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

type Provider interface {
    Save(schoolID int, filename string, data []byte) (string, error)
}

type LocalStorage struct {
    BaseDir string
}

func (l LocalStorage) Save(schoolID int, filename string, data []byte) (string, error) {
    safe := sanitizeFilename(filename)
    dir := filepath.Join(l.BaseDir, intToString(schoolID))
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return "", err
    }
    name := intToString(int(time.Now().Unix())) + "_" + safe
    path := filepath.Join(dir, name)
    if err := os.WriteFile(path, data, 0o644); err != nil {
        return "", err
    }
    return filepath.ToSlash(filepath.Join(intToString(schoolID), name)), nil
}

type StubStorage struct {
    Provider string
}

func (s StubStorage) Save(_ int, _ string, _ []byte) (string, error) {
    return "", errors.New("storage provider not configured")
}

func New(provider, localDir string) Provider {
    switch strings.ToLower(provider) {
    case "local", "":
        return LocalStorage{BaseDir: localDir}
    case "r2", "supabase":
        return StubStorage{Provider: provider}
    default:
        return StubStorage{Provider: provider}
    }
}

func sanitizeFilename(name string) string {
    cleaned := strings.Map(func(r rune) rune {
        if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
            return r
        }
        return '_'
    }, name)
    if cleaned == "" {
        return "file.xlsx"
    }
    return cleaned
}

func intToString(value int) string {
    return strconv.Itoa(value)
}
