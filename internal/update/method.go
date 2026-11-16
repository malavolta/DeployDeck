package update

import (
	"path"
	"regexp"
	"strings"
)

// ReleasesURL is where users can download a release manually.
const ReleasesURL = "https://github.com/malavolta/DeployDeck/releases/latest"

const goInstallPkg = "github.com/malavolta/DeployDeck/cmd/deploydeck"

// tagPattern is the only tag shape allowed into an upgrade argv (ADR-6).
var tagPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)

// Method is how the running executable was installed. The zero value is
// MethodManual.
type Method int

const (
	MethodManual Method = iota
	MethodBrew
	MethodScoop
	MethodGoInstall
)

// Tool is the executable that performs the upgrade, "" for MethodManual.
func (m Method) Tool() string {
	return [...]string{"", "brew", "scoop", "go"}[m]
}

// DetectMethod classifies exePath purely by string matching on a
// slash-normalized path (never filepath). Order: Brew, Scoop, GoInstall.
func DetectMethod(exePath, goos string, getenv func(string) string) Method {
	windows := goos == "windows"
	norm := func(p string) string {
		if p = strings.ReplaceAll(p, `\`, "/"); windows {
			p = strings.ToLower(p)
		}
		return strings.TrimRight(p, "/")
	}
	exe := norm(exePath)

	if !windows && (strings.Contains(exe, "/Caskroom/") || strings.Contains(exe, "/Cellar/") ||
		strings.HasPrefix(exe, "/opt/homebrew/") || strings.HasPrefix(exe, "/home/linuxbrew/.linuxbrew/")) {
		return MethodBrew
	}
	if windows {
		if strings.Contains(exe, "/scoop/apps/") || strings.Contains(exe, "/scoop/shims/") {
			return MethodScoop
		}
		if root := norm(getenv("SCOOP")); root != "" && strings.HasPrefix(exe, root+"/apps/") {
			return MethodScoop
		}
	}

	dir := path.Dir(exe)
	if gobin := norm(getenv("GOBIN")); gobin != "" && dir == gobin {
		return MethodGoInstall
	}
	sep, home := ":", getenv("HOME")
	if windows {
		sep, home = ";", getenv("USERPROFILE")
	}
	if gopath := getenv("GOPATH"); gopath != "" {
		first := strings.SplitN(gopath, sep, 2)[0]
		if dir == norm(first)+"/bin" {
			return MethodGoInstall
		}
	}
	if home != "" && dir == norm(home)+"/go/bin" {
		return MethodGoInstall
	}
	return MethodManual
}

// UpgradeArgs builds the upgrade argv; nil for Manual or a tag that is not
// strictly vX.Y.Z (ADR-6).
func UpgradeArgs(m Method, latest string) []string {
	switch m {
	case MethodBrew:
		return []string{"brew", "upgrade", "--cask", "deploydeck"}
	case MethodScoop:
		return []string{"scoop", "update", "deploydeck"}
	case MethodGoInstall:
		if !tagPattern.MatchString(latest) {
			return nil
		}
		return []string{"go", "install", goInstallPkg + "@v" + strings.TrimPrefix(latest, "v")}
	}
	return nil
}
