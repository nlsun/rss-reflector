package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultFilePerm os.FileMode = 0644
	DefaultDirPerm  os.FileMode = 0755
)

func FileExists(path string) (bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// List of files in directory sorted by oldest first.
func FilesSortedByOldest(path string) ([]string, error) {
	outputFiles := []string{}
	mixedfiles, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	sort.Slice(mixedfiles, func(i, j int) bool {
		return mixedfiles[i].ModTime().Before(mixedfiles[j].ModTime())
	})

	for _, file := range mixedfiles {
		if file.Mode().IsRegular() {
			outputFiles = append(outputFiles, file.Name())
		}
	}
	return outputFiles, nil
}

// Returns empty string if file not found.
func FindFileWithPrefix(pathPrefix string) (string, error) {
	dir := filepath.Dir(pathPrefix)
	base := filepath.Base(pathPrefix)

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), base) {
			return filepath.Join(dir, file.Name()), nil
		}
	}
	return "", nil
}
