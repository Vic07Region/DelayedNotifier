package migrator

import (
	"fmt"
	"strings"
)

func normalizePath(path string) string {
	if strings.HasPrefix(path, "file://") {
		return path
	}
	return fmt.Sprintf("file://%s", path)
}
