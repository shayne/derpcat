package derphole

import (
	"errors"
	"os"
	"path/filepath"
)

func ResolveOutputPath(outputPath, suggested string) (string, error) {
	name := filepath.Base(suggested)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "", errors.New("missing suggested filename")
	}
	if outputPath == "" {
		return name, nil
	}

	info, err := os.Stat(outputPath)
	if err == nil && info.IsDir() {
		return filepath.Join(outputPath, name), nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return outputPath, nil
}
