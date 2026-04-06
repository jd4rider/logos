package appenv

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	unixAppSlug   = "logos"
	desktopAppDir = "Logos AI"
)

func DataDir() string {
	switch runtime.GOOS {
	case "windows":
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, desktopAppDir)
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", desktopAppDir)
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", unixAppSlug)
	}

	return filepath.Join(".", unixAppSlug)
}

func ConfigDir() string {
	switch runtime.GOOS {
	case "windows":
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, desktopAppDir)
		}
	case "darwin":
		return DataDir()
	}

	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", unixAppSlug)
	}

	return filepath.Join(".", "."+unixAppSlug)
}

func CacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			return filepath.Join(dir, desktopAppDir)
		}
		return filepath.Join(dir, unixAppSlug)
	}

	return filepath.Join(DataDir(), "cache")
}

func BinDir() string {
	return filepath.Join(DataDir(), "bin")
}

func VenvDir() string {
	return filepath.Join(DataDir(), "venv")
}

func VenvBinDir() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(VenvDir(), "Scripts")
	}
	return filepath.Join(VenvDir(), "bin")
}

func VenvPythonPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(VenvBinDir(), "python.exe")
	}
	return filepath.Join(VenvBinDir(), "python")
}

func PythonPointerPath() string {
	return filepath.Join(DataDir(), "python_interp")
}

func PiperDir() string {
	return filepath.Join(DataDir(), "piper")
}

func KokoroDir() string {
	return filepath.Join(DataDir(), "kokoro")
}
