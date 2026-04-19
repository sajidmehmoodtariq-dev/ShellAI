package platformdb

import (
	"os"
	"path/filepath"
	"runtime"
)

func CurrentOS() string {
	return runtime.GOOS
}

func CoreCommandsPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join("db", "commands_windows.json")
	case "darwin":
		return filepath.Join("db", "commands_darwin.json")
	default:
		return filepath.Join("db", "commands_linux.json")
	}
}

func UserCommandsPath() string {
	home, _ := os.UserHomeDir()
	name := "user_commands_" + runtime.GOOS + ".json"
	return filepath.Join(home, ".config", "shellai", name)
}
