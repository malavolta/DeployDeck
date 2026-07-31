package delta_test

import (
	"reflect"
	"testing"

	"github.com/malavolta/DeployDeck/internal/delta"
)

// TestSummarize_TableDriven pins HU-008's five ACs in one table: per-type
// counts, the Empty warning, destructive changes kept SEPARATE (never
// inflating additive counts), the sensitive-metadata warning, and files
// outside the configured sourceDirs.
func TestSummarize_TableDriven(t *testing.T) {
	tests := []struct {
		name               string
		pkg                delta.Package
		destructive        delta.Package
		changedFiles       []string
		sourceDirs         []string
		wantTypes          []delta.MetadataTypeSummary
		wantDestructive    []delta.MetadataTypeSummary
		wantHasDestructive bool
		wantSensitive      []string
		wantOutside        []string
		wantEmpty          bool
	}{
		{
			name: "normal package with multiple types shows a count per type",
			pkg: delta.Package{Types: []delta.PackageType{
				{Name: "ApexClass", Members: []string{"AccountService", "ContactService"}},
				{Name: "ApexTrigger", Members: []string{"AccountTrigger"}},
			}},
			wantTypes: []delta.MetadataTypeSummary{
				{Name: "ApexClass", Count: 2},
				{Name: "ApexTrigger", Count: 1},
			},
			wantEmpty: false,
		},
		{
			name:      "empty package is clearly warned via Empty",
			pkg:       delta.Package{},
			wantEmpty: true,
		},
		{
			name: "destructive changes are kept separate and never inflate additive counts",
			pkg: delta.Package{Types: []delta.PackageType{
				{Name: "ApexClass", Members: []string{"AccountService"}},
			}},
			destructive: delta.Package{Types: []delta.PackageType{
				{Name: "ApexClass", Members: []string{"OldService"}},
			}},
			wantTypes: []delta.MetadataTypeSummary{
				{Name: "ApexClass", Count: 1},
			},
			wantDestructive: []delta.MetadataTypeSummary{
				{Name: "ApexClass", Count: 1},
			},
			wantHasDestructive: true,
		},
		{
			name: "sensitive metadata types are flagged",
			pkg: delta.Package{Types: []delta.PackageType{
				{Name: "Profile", Members: []string{"Admin"}},
				{Name: "PermissionSet", Members: []string{"CustomPerm"}},
				{Name: "ApexClass", Members: []string{"NotSensitive"}},
			}},
			wantTypes: []delta.MetadataTypeSummary{
				{Name: "Profile", Count: 1},
				{Name: "PermissionSet", Count: 1},
				{Name: "ApexClass", Count: 1},
			},
			wantSensitive: []string{"Profile", "PermissionSet"},
		},
		{
			name: "sensitive destructive-only type is still flagged",
			pkg: delta.Package{Types: []delta.PackageType{
				{Name: "ApexClass", Members: []string{"A"}},
			}},
			destructive: delta.Package{Types: []delta.PackageType{
				{Name: "Flow", Members: []string{"Old_Flow"}},
			}},
			wantTypes: []delta.MetadataTypeSummary{
				{Name: "ApexClass", Count: 1},
			},
			wantDestructive: []delta.MetadataTypeSummary{
				{Name: "Flow", Count: 1},
			},
			wantHasDestructive: true,
			wantSensitive:      []string{"Flow"},
		},
		{
			name:         "files outside configured sourceDirs are listed",
			pkg:          delta.Package{Types: []delta.PackageType{{Name: "ApexClass", Members: []string{"A"}}}},
			changedFiles: []string{"force-app/main/default/classes/A.cls", "README.md", "scripts/deploy.sh"},
			sourceDirs:   []string{"force-app"},
			wantTypes:    []delta.MetadataTypeSummary{{Name: "ApexClass", Count: 1}},
			wantOutside:  []string{"README.md", "scripts/deploy.sh"},
		},
		{
			name:         "a file exactly matching sourceDirs is not outside",
			pkg:          delta.Package{Types: []delta.PackageType{{Name: "ApexClass", Members: []string{"A"}}}},
			changedFiles: []string{"force-app/main/default/classes/A.cls", "unpackaged/pre/destructiveChanges.xml"},
			sourceDirs:   []string{"force-app", "unpackaged"},
			wantTypes:    []delta.MetadataTypeSummary{{Name: "ApexClass", Count: 1}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := delta.Summarize(tt.pkg, tt.destructive, tt.changedFiles, tt.sourceDirs)

			if !reflect.DeepEqual(got.Types, tt.wantTypes) {
				t.Errorf("Types = %+v, want %+v", got.Types, tt.wantTypes)
			}
			if !reflect.DeepEqual(got.DestructiveTypes, tt.wantDestructive) {
				t.Errorf("DestructiveTypes = %+v, want %+v", got.DestructiveTypes, tt.wantDestructive)
			}
			if got.HasDestructive != tt.wantHasDestructive {
				t.Errorf("HasDestructive = %v, want %v", got.HasDestructive, tt.wantHasDestructive)
			}
			if !reflect.DeepEqual(got.SensitiveTypes, tt.wantSensitive) {
				t.Errorf("SensitiveTypes = %v, want %v", got.SensitiveTypes, tt.wantSensitive)
			}
			if !reflect.DeepEqual(got.OutsideSourceDirs, tt.wantOutside) {
				t.Errorf("OutsideSourceDirs = %v, want %v", got.OutsideSourceDirs, tt.wantOutside)
			}
			if got.Empty != tt.wantEmpty {
				t.Errorf("Empty = %v, want %v", got.Empty, tt.wantEmpty)
			}
		})
	}
}
