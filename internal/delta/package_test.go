package delta_test

import (
	"testing"

	"deploydeck/internal/delta"
)

const packageXMLFixture = `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>AccountService</members>
        <members>ContactService</members>
        <name>ApexClass</name>
    </types>
    <types>
        <members>Admin</members>
        <name>Profile</name>
    </types>
    <version>60.0</version>
</Package>`

const emptyPackageXMLFixture = `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <version>60.0</version>
</Package>`

const destructiveChangesXMLFixture = `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>OldTrigger</members>
        <name>ApexTrigger</name>
    </types>
    <version>60.0</version>
</Package>`

// TestParsePackage_ParsesTypesAndMembers proves ParsePackage decodes
// package.xml's <types>/<members>/<name> shape into Package, preserving
// declaration order and every member per type.
func TestParsePackage_ParsesTypesAndMembers(t *testing.T) {
	pkg, err := delta.ParsePackage([]byte(packageXMLFixture))
	if err != nil {
		t.Fatalf("ParsePackage() unexpected error: %v", err)
	}
	if len(pkg.Types) != 2 {
		t.Fatalf("expected 2 types, got %d: %+v", len(pkg.Types), pkg.Types)
	}
	if pkg.Types[0].Name != "ApexClass" || len(pkg.Types[0].Members) != 2 {
		t.Errorf("Types[0] = %+v, want ApexClass with 2 members", pkg.Types[0])
	}
	if pkg.Types[0].Members[0] != "AccountService" || pkg.Types[0].Members[1] != "ContactService" {
		t.Errorf("Types[0].Members = %v, want [AccountService ContactService]", pkg.Types[0].Members)
	}
	if pkg.Types[1].Name != "Profile" || len(pkg.Types[1].Members) != 1 {
		t.Errorf("Types[1] = %+v, want Profile with 1 member", pkg.Types[1])
	}
}

// TestParsePackage_EmptyPackageHasNoTypes proves a package.xml with no
// <types> blocks (the empty-delta case HU-007/HU-008 warn about) parses to
// zero Types rather than erroring.
func TestParsePackage_EmptyPackageHasNoTypes(t *testing.T) {
	pkg, err := delta.ParsePackage([]byte(emptyPackageXMLFixture))
	if err != nil {
		t.Fatalf("ParsePackage() unexpected error: %v", err)
	}
	if len(pkg.Types) != 0 {
		t.Errorf("expected zero types for an empty package.xml, got %+v", pkg.Types)
	}
}

// TestParsePackage_InvalidXMLErrors proves malformed input is reported as
// an error rather than a silently empty Package.
func TestParsePackage_InvalidXMLErrors(t *testing.T) {
	if _, err := delta.ParsePackage([]byte("not xml")); err == nil {
		t.Fatal("expected an error parsing invalid XML")
	}
}

// TestParseDestructive_ParsesTypesAndMembers proves ParseDestructive
// decodes destructiveChanges.xml with the same schema as package.xml.
func TestParseDestructive_ParsesTypesAndMembers(t *testing.T) {
	pkg, err := delta.ParseDestructive([]byte(destructiveChangesXMLFixture))
	if err != nil {
		t.Fatalf("ParseDestructive() unexpected error: %v", err)
	}
	if len(pkg.Types) != 1 || pkg.Types[0].Name != "ApexTrigger" || len(pkg.Types[0].Members) != 1 {
		t.Errorf("Types = %+v, want one ApexTrigger with 1 member", pkg.Types)
	}
	if pkg.Types[0].Members[0] != "OldTrigger" {
		t.Errorf("Types[0].Members = %v, want [OldTrigger]", pkg.Types[0].Members)
	}
}

// TestParseDestructive_InvalidXMLErrors proves malformed destructive input
// is also reported as an error.
func TestParseDestructive_InvalidXMLErrors(t *testing.T) {
	if _, err := delta.ParseDestructive([]byte("not xml")); err == nil {
		t.Fatal("expected an error parsing invalid XML")
	}
}
