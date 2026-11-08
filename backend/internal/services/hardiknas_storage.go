package services

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"cabdin-bangkalan-backend/internal/logging"
)

const (
	hardiknasStorageBaseDir       = "lomba-hardiknas"
	hardiknasOriginalRelativeDir  = hardiknasStorageBaseDir + "/original"
	hardiknasThumbnailRelativeDir = hardiknasStorageBaseDir + "/thumbnail"
)

type HardiknasStorageDiagnostics struct {
	StorageRoot        string `json:"storageRoot"`
	OriginalDir        string `json:"originalDir"`
	ThumbnailDir       string `json:"thumbnailDir"`
	RootExists         bool   `json:"rootExists"`
	RootWritable       bool   `json:"rootWritable"`
	OriginalFileCount  int    `json:"originalFileCount"`
	ThumbnailFileCount int    `json:"thumbnailFileCount"`
}

func LogHardiknasStorageStartup(storageLocalDir string) {
	rawStorageDir := strings.TrimSpace(os.Getenv("STORAGE_LOCAL_DIR"))
	effectiveStorageDir := strings.TrimSpace(storageLocalDir)
	if effectiveStorageDir == "" {
		effectiveStorageDir = "./uploads"
	}

	storageRoot, err := resolveHardiknasStorageRoot(effectiveStorageDir)
	if err != nil {
		logging.Warnf("hardiknas", "storage_config", "storage_root=%q err=%v", effectiveStorageDir, err)
		return
	}
	rootExistsBefore := hardiknasDirExists(storageRoot)
	if err := os.MkdirAll(storageRoot, 0o755); err != nil {
		logging.Warnf("hardiknas", "storage_config", "storage_root=%q mkdir_err=%v", storageRoot, err)
	} else {
		rootExistsBefore = true
	}
	rootWritable := hardiknasDirWritable(storageRoot)

	logging.Infof(
		"hardiknas",
		"storage_config",
		"storage_root=%q configured_storage_local_dir=%q root_exists=%t root_writable=%t",
		storageRoot,
		effectiveStorageDir,
		rootExistsBefore,
		rootWritable,
	)
	if !rootWritable {
		logging.Warnf("hardiknas", "storage_config", "msg=%q storage_root=%q", "Storage root is not writable. Hardiknas uploads may fail.", storageRoot)
	}

	if rawStorageDir == "" {
		logging.Warnf(
			"hardiknas",
			"storage_config",
			"msg=%q storage_root=%q",
			"STORAGE_LOCAL_DIR is empty; using ./uploads. This is not safe for staging/production.",
			storageRoot,
		)
	}

	if hardiknasEnvLooksProductionLike() && hardiknasStorageRootLooksUnsafe(effectiveStorageDir, storageRoot) {
		logging.Warnf(
			"hardiknas",
			"storage_config",
			"msg=%q storage_root=%q app_env=%q environment=%q gin_mode=%q",
			"Storage root is using a non-persistent local path. Set STORAGE_LOCAL_DIR to a mounted persistent volume such as /data/uploads for staging/production.",
			storageRoot,
			strings.TrimSpace(os.Getenv("APP_ENV")),
			strings.TrimSpace(os.Getenv("ENVIRONMENT")),
			strings.TrimSpace(os.Getenv("GIN_MODE")),
		)
	}
}

func (s HardiknasService) StorageDiagnostics() HardiknasStorageDiagnostics {
	storageRoot, err := resolveHardiknasStorageRoot(s.StorageLocalDir)
	if err != nil {
		storageRoot = strings.TrimSpace(s.StorageLocalDir)
	}
	originalDir := ""
	thumbnailDir := ""
	if storageRoot != "" {
		originalDir = filepath.Join(storageRoot, filepath.FromSlash(hardiknasOriginalRelativeDir))
		thumbnailDir = filepath.Join(storageRoot, filepath.FromSlash(hardiknasThumbnailRelativeDir))
	}
	rootExists := hardiknasDirExists(storageRoot)
	rootWritable := hardiknasDirWritable(storageRoot)
	return HardiknasStorageDiagnostics{
		StorageRoot:        storageRoot,
		OriginalDir:        originalDir,
		ThumbnailDir:       thumbnailDir,
		RootExists:         rootExists,
		RootWritable:       rootWritable,
		OriginalFileCount:  hardiknasCountFiles(originalDir),
		ThumbnailFileCount: hardiknasCountFiles(thumbnailDir),
	}
}

func resolveHardiknasStorageRoot(storageLocalDir string) (string, error) {
	root := strings.TrimSpace(storageLocalDir)
	if root == "" {
		return "", ErrHardiknasStorageMisconfig
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absoluteRoot), nil
}

func normalizeHardiknasStoredPath(storedPath string) (string, error) {
	raw := strings.ReplaceAll(strings.TrimSpace(storedPath), "\\", "/")
	if raw == "" {
		return "", ErrHardiknasFileNotAvailable
	}

	candidate := ""
	switch {
	case strings.HasPrefix(raw, hardiknasStorageBaseDir+"/"):
		candidate = raw
	case strings.HasPrefix(raw, "./"+hardiknasStorageBaseDir+"/"):
		candidate = strings.TrimPrefix(raw, "./")
	case strings.HasPrefix(raw, "uploads/"+hardiknasStorageBaseDir+"/"):
		candidate = strings.TrimPrefix(raw, "uploads/")
	case strings.HasPrefix(raw, "./uploads/"+hardiknasStorageBaseDir+"/"):
		candidate = strings.TrimPrefix(raw, "./uploads/")
	case strings.HasPrefix(raw, "/app/uploads/"+hardiknasStorageBaseDir+"/"):
		candidate = strings.TrimPrefix(raw, "/app/uploads/")
	default:
		if hardiknasPathLooksAbsolute(raw) {
			if idx := strings.Index(raw, "/"+hardiknasStorageBaseDir+"/"); idx >= 0 {
				candidate = strings.TrimPrefix(raw[idx:], "/")
			}
		}
	}

	if candidate == "" {
		return "", ErrHardiknasFileNotAvailable
	}

	cleaned := path.Clean(candidate)
	switch {
	case cleaned == ".", cleaned == "":
		return "", ErrHardiknasFileNotAvailable
	case strings.HasPrefix(cleaned, "/"):
		return "", ErrHardiknasFileNotAvailable
	case cleaned == "..", strings.HasPrefix(cleaned, "../"):
		return "", ErrHardiknasFileNotAvailable
	case cleaned != hardiknasOriginalRelativeDir &&
		cleaned != hardiknasThumbnailRelativeDir &&
		!strings.HasPrefix(cleaned, hardiknasOriginalRelativeDir+"/") &&
		!strings.HasPrefix(cleaned, hardiknasThumbnailRelativeDir+"/"):
		return "", ErrHardiknasFileNotAvailable
	default:
		return cleaned, nil
	}
}

func hardiknasPathLooksAbsolute(raw string) bool {
	if strings.HasPrefix(raw, "/") {
		return true
	}
	if len(raw) >= 3 && raw[1] == ':' && raw[2] == '/' {
		return true
	}
	return filepath.IsAbs(filepath.FromSlash(raw))
}

func hardiknasDirExists(target string) bool {
	if strings.TrimSpace(target) == "" {
		return false
	}
	info, err := os.Stat(target)
	return err == nil && info.IsDir()
}

func hardiknasDirWritable(target string) bool {
	if strings.TrimSpace(target) == "" {
		return false
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return false
	}
	file, err := os.CreateTemp(target, ".hardiknas-write-check-*")
	if err != nil {
		return false
	}
	name := file.Name()
	if closeErr := file.Close(); closeErr != nil {
		_ = os.Remove(name)
		return false
	}
	return os.Remove(name) == nil
}

func hardiknasCountFiles(target string) int {
	entries, err := os.ReadDir(target)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			count++
		}
	}
	return count
}

func hardiknasEnvLooksProductionLike() bool {
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	ginMode := strings.ToLower(strings.TrimSpace(os.Getenv("GIN_MODE")))

	return appEnv == "production" ||
		appEnv == "staging" ||
		environment == "production" ||
		environment == "staging" ||
		ginMode == "release"
}

func hardiknasStorageRootLooksUnsafe(configuredPath string, resolvedRoot string) bool {
	cleanConfigured := filepath.ToSlash(filepath.Clean(strings.TrimSpace(configuredPath)))
	cleanResolved := filepath.ToSlash(filepath.Clean(strings.TrimSpace(resolvedRoot)))

	switch cleanConfigured {
	case "", ".", "./uploads", "uploads":
		return true
	}
	if cleanResolved == "" {
		return true
	}
	return cleanResolved == "/app/uploads"
}
