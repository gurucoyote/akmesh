package appname

import "testing"

// TestOrgFollowsTheName: the default issuer has to move with the build name,
// otherwise a bssdsh build issues certificates that say akmesh.
func TestOrgFollowsTheName(t *testing.T) {
	old := Name
	defer func() { Name = old }()
	Name = "bssdsh"
	if got := Org(); got != "bssdsh mkcert" {
		t.Fatalf("Org() = %q", got)
	}
}

// TestDefaultNameIsAkmesh pins the default: the battery and the help texts of a
// plain build say akmesh, and that is the name readers know.
func TestDefaultNameIsAkmesh(t *testing.T) {
	if Name != "akmesh" {
		t.Fatalf("default Name = %q, want akmesh", Name)
	}
}
