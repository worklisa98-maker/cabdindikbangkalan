package assets

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	rawassets "cabdin-bangkalan-backend/assets"
)

var (
	FS embed.FS = rawassets.FS

	ErrAssetNotFound    = errors.New("asset not found")
	ErrInvalidAssetName = errors.New("invalid asset name")
)

func ReadAsset(name string) ([]byte, error) {
	assetName, err := normalizeAssetName(name)
	if err != nil {
		return nil, err
	}

	content, err := fs.ReadFile(FS, assetName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrAssetNotFound, assetName)
		}
		return nil, err
	}

	return content, nil
}

func OpenAsset(name string) (io.ReadSeeker, int64, error) {
	content, err := ReadAsset(name)
	if err != nil {
		return nil, 0, err
	}
	return bytes.NewReader(content), int64(len(content)), nil
}

func normalizeAssetName(name string) (string, error) {
	assetName := strings.TrimSpace(name)
	if assetName == "" || assetName == "." || assetName == ".." {
		return "", ErrInvalidAssetName
	}
	if strings.Contains(assetName, "/") || strings.Contains(assetName, "\\") || filepath.Base(assetName) != assetName {
		return "", fmt.Errorf("%w: %s", ErrInvalidAssetName, name)
	}
	return assetName, nil
}
