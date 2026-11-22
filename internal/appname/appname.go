// Package appname carries the name this build answers to.
//
// The repository is akmesh, the binary need not be: the same sources build a
// tool called bssdsh, and everything a user reads - the command list, the
// version lines, the issuer of a test certificate - says that name. The module
// path stays akmesh/internal; a name and an import path are two different
// things.
//
// Name is a var, not a const, because only a var can be set at link time:
//
//	go build -ldflags "-X akmesh/internal/appname.Name=bssdsh"
//
// scripts/build-bb.sh passes AKMESH_NAME through this way; with nothing given,
// the tool is akmesh.
package appname

// Name is the product name of this build.
var Name = "akmesh"

// Org is the organisation a certificate made by this build carries when no
// issuer is asked for: "akmesh mkcert", and with another build name that name.
// It is a default, not a rule - mkcert -org replaces it for a new CA.
func Org() string { return Name + " mkcert" }
