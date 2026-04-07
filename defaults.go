package ipcheck

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

const (
	geoCityDBName = "GeoLite2-City.mmdb"
	geoASNDBName  = "GeoLite2-ASN.mmdb"
)

//go:embed config-ex.ini
var defaultIPCheckConfig string

//go:embed geo-ex.ini
var defaultGeoConfig string

type appPaths struct {
	baseDir       string
	ipCheckConfig string
	ipCheckConfigEx string
	geoConfig     string
	geoConfigEx   string
	geoVersion    string
	geoCityDB     string
	geoASNDB      string
}

func newAppPaths() appPaths {
	baseDir := appBaseDir()
	return appPaths{
		baseDir:       baseDir,
		ipCheckConfig: filepath.Join(baseDir, "config.ini"),
		ipCheckConfigEx: filepath.Join(baseDir, "config-ex.ini"),
		geoConfig:     filepath.Join(baseDir, "geo.ini"),
		geoConfigEx:   filepath.Join(baseDir, "geo-ex.ini"),
		geoVersion:    filepath.Join(baseDir, ".geo_version"),
		geoCityDB:     filepath.Join(baseDir, geoCityDBName),
		geoASNDB:      filepath.Join(baseDir, geoASNDBName),
	}
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return filepath.Dir(exe)
}

func appBaseDir() string {
	if home := strings.TrimSpace(os.Getenv("IPCHECK_HOME")); home != "" {
		if abs, err := filepath.Abs(home); err == nil {
			return abs
		}
		return home
	}
	return executableDir()
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	"Mozilla/5.0 (Linux; Android 13; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
}
