package delta

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// Package is the parsed shape shared by package.xml and
// destructiveChanges.xml — both are Metadata API "Package" manifests; only
// their role (additive vs destructive) differs by which file a caller
// parsed, not by schema.
type Package struct {
	XMLName xml.Name      `xml:"Package"`
	Types   []PackageType `xml:"types"`
	Version string        `xml:"version"`
}

// PackageType is one <types> block: a metadata type name and its member
// full names, in document order.
type PackageType struct {
	Members []string `xml:"members"`
	Name    string   `xml:"name"`
}

// ParsePackage parses package.xml's additive manifest.
func ParsePackage(data []byte) (Package, error) {
	pkg, err := parsePackageXML(data)
	if err != nil {
		return Package{}, fmt.Errorf("delta: parsing package.xml: %w", err)
	}
	return pkg, nil
}

// ParseDestructive parses destructiveChanges.xml. It shares package.xml's
// exact schema (both are Metadata API "Package" documents) — only the file
// a caller passes in distinguishes "additive" from "destructive".
func ParseDestructive(data []byte) (Package, error) {
	pkg, err := parsePackageXML(data)
	if err != nil {
		return Package{}, fmt.Errorf("delta: parsing destructiveChanges.xml: %w", err)
	}
	return pkg, nil
}

func parsePackageXML(data []byte) (Package, error) {
	var pkg Package
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return Package{}, err
	}
	return pkg, nil
}

// MetadataTypeSummary is one metadata type's member count (HU-008's
// suggested model, docs/HISTORIAS.md HU-008).
type MetadataTypeSummary struct {
	Name  string
	Count int
}

// PackageSummary is HU-008's package.xml/destructiveChanges.xml review
// model, shown before Salesforce validation runs.
type PackageSummary struct {
	// Types are package.xml's additive per-type member counts.
	Types []MetadataTypeSummary
	// DestructiveTypes are destructiveChanges.xml's per-type member
	// counts, kept SEPARATE from Types — a metadata type present in both
	// files never has its destructive members folded into its additive
	// Count.
	DestructiveTypes []MetadataTypeSummary
	// HasDestructive is true when destructiveChanges.xml carried at least
	// one member.
	HasDestructive bool
	// SensitiveTypes lists which sensitiveMetadataTypes appear, additive
	// or destructive, for the pre-validation warning.
	SensitiveTypes []string
	// OutsideSourceDirs lists changedFiles that fall outside every
	// configured sourceDirs entry.
	OutsideSourceDirs []string
	// Empty is true when package.xml (additive) has zero total members.
	Empty bool
}

// sensitiveMetadataTypes are the metadata types HU-008 flags with a
// warning before the user continues (docs/HISTORIAS.md HU-008 AC).
var sensitiveMetadataTypes = map[string]bool{
	"Profile":       true,
	"PermissionSet": true,
	"Flow":          true,
	"CustomObject":  true,
	"CustomField":   true,
}

// Summarize builds a PackageSummary from the parsed additive (pkg) and
// destructive packages, plus the raw changed-files list (from
// Service.ChangedFiles) and the configured delta.sourceDirs.
func Summarize(pkg, destructive Package, changedFiles, sourceDirs []string) PackageSummary {
	types, additiveTotal := summarizeTypes(pkg)
	destructiveTypes, destructiveTotal := summarizeTypes(destructive)

	return PackageSummary{
		Types:             types,
		DestructiveTypes:  destructiveTypes,
		HasDestructive:    destructiveTotal > 0,
		SensitiveTypes:    sensitiveTypesIn(pkg, destructive),
		OutsideSourceDirs: outsideSourceDirs(changedFiles, sourceDirs),
		Empty:             additiveTotal == 0,
	}
}

// summarizeTypes converts a Package's <types> blocks into per-type member
// counts, preserving source (document) order, plus the total member count
// across every type.
func summarizeTypes(pkg Package) ([]MetadataTypeSummary, int) {
	var summaries []MetadataTypeSummary
	total := 0
	for _, t := range pkg.Types {
		summaries = append(summaries, MetadataTypeSummary{Name: t.Name, Count: len(t.Members)})
		total += len(t.Members)
	}
	return summaries, total
}

// sensitiveTypesIn scans every given Package's types (additive and
// destructive alike — a sensitive type being REMOVED is exactly as worth
// flagging as one being added) and returns the sensitive ones present,
// each listed once, in first-seen order.
func sensitiveTypesIn(pkgs ...Package) []string {
	seen := make(map[string]bool)
	var sensitive []string
	for _, pkg := range pkgs {
		for _, t := range pkg.Types {
			if sensitiveMetadataTypes[t.Name] && !seen[t.Name] {
				seen[t.Name] = true
				sensitive = append(sensitive, t.Name)
			}
		}
	}
	return sensitive
}

func outsideSourceDirs(changedFiles, sourceDirs []string) []string {
	var outside []string
	for _, f := range changedFiles {
		if !underAnySourceDir(f, sourceDirs) {
			outside = append(outside, f)
		}
	}
	return outside
}

// underAnySourceDir reports whether file sits under one of sourceDirs,
// matching either an exact directory-path equal or a "<dir>/" prefix (a
// trailing slash on a configured dir is tolerated).
func underAnySourceDir(file string, sourceDirs []string) bool {
	for _, dir := range sourceDirs {
		dir = strings.TrimSuffix(dir, "/")
		if file == dir || strings.HasPrefix(file, dir+"/") {
			return true
		}
	}
	return false
}
