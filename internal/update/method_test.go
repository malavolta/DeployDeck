package update_test

import (
	"reflect"
	"testing"

	"github.com/malavolta/DeployDeck/internal/update"
)

func TestDetectMethod(t *testing.T) {
	tests := []struct {
		name string
		exe  string
		goos string
		env  map[string]string
		want update.Method
	}{
		{"brew caskroom", "/opt/homebrew/Caskroom/deploydeck/1.0.0/deploydeck", "darwin", nil, update.MethodBrew},
		{"linuxbrew", "/home/linuxbrew/.linuxbrew/bin/deploydeck", "linux", nil, update.MethodBrew},
		{"intel cellar", "/usr/local/Cellar/deploydeck/1.0.0/bin/deploydeck", "darwin", nil, update.MethodBrew},
		{"scoop apps", `C:\Users\u\scoop\apps\deploydeck\current\deploydeck.exe`, "windows", nil, update.MethodScoop},
		{"scoop mixed case", `C:\Users\u\SCOOP\Apps\deploydeck\current\deploydeck.exe`, "windows", nil, update.MethodScoop},
		{"scoop custom root", `D:\tools\sc\apps\deploydeck\current\deploydeck.exe`, "windows", map[string]string{"SCOOP": `D:\tools\sc`}, update.MethodScoop},
		{"go default", "/home/u/go/bin/deploydeck", "linux", map[string]string{"HOME": "/home/u"}, update.MethodGoInstall},
		{"go gobin", "/srv/bin/deploydeck", "linux", map[string]string{"GOBIN": "/srv/bin"}, update.MethodGoInstall},
		{"go gopath list", "/a/bin/deploydeck", "linux", map[string]string{"GOPATH": "/a:/b"}, update.MethodGoInstall},
		{"usr local bin", "/usr/local/bin/deploydeck", "linux", map[string]string{"HOME": "/home/u"}, update.MethodManual},
		{"empty", "", "linux", nil, update.MethodManual},
		{"gobin inside brew prefix", "/opt/homebrew/gobin/deploydeck", "darwin", map[string]string{"GOBIN": "/opt/homebrew/gobin"}, update.MethodBrew},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := update.DetectMethod(tt.exe, tt.goos, func(k string) string { return tt.env[k] })
			if got != tt.want {
				t.Fatalf("DetectMethod(%q) = %v, want %v", tt.exe, got, tt.want)
			}
		})
	}
}

func TestUpgradeArgs(t *testing.T) {
	const pkg = "github.com/malavolta/DeployDeck/cmd/deploydeck"
	tests := []struct {
		name   string
		m      update.Method
		latest string
		want   []string
	}{
		{"brew", update.MethodBrew, "v1.3.0", []string{"brew", "upgrade", "--cask", "deploydeck"}},
		{"scoop", update.MethodScoop, "v1.3.0", []string{"scoop", "update", "deploydeck"}},
		{"go tag", update.MethodGoInstall, "v1.3.0", []string{"go", "install", pkg + "@v1.3.0"}},
		{"go tag without v", update.MethodGoInstall, "1.3.0", []string{"go", "install", pkg + "@v1.3.0"}},
		{"invalid short", update.MethodGoInstall, "v1.2", nil},
		{"invalid injection", update.MethodGoInstall, "v1.2.3 && x", nil},
		{"invalid flag", update.MethodGoInstall, "-x", nil},
		{"invalid empty", update.MethodGoInstall, "", nil},
		{"manual", update.MethodManual, "v1.3.0", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := update.UpgradeArgs(tt.m, tt.latest)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("UpgradeArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
