//go:build !windows

package login

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The expected hashes were produced on this machine by openssl (3.5.5) and
// perl's crypt against glibc, so the test compares against the system and not
// against this code's own idea of the format.
var hashCases = []struct {
	scheme   string
	hash     string
	password string
}{
	{"$6$", "$6$saltstring$svn8UoSVapNtMuq1ukKS4tPQd8iKwSMHWjl/O817G3uBnIFNjnQJuesI68u4OTLiBFdcbYEdFCoEOfaS35inz1", "Hello world!"},
	{"$6$", "$6$saltsaltsalt$Oq7xKWOqvAuIWaYu0s.FKNG4aRu4ZRfWgMA//8s9bUG.pUCJfZ2vgx6EHv3itYosTanlw5D5IstcmXqwCIXtc.", "anderes"},
	{"$6$ rounds", "$6$rounds=10000$roundsalt$6JmslYiK.ctpQYSv9pnwYGYgkxM7jDo7z6vWU/Dn1AE4dwDvEi1m5a4bfkGjbQUXSa..tl/xHoEZ8ZbicyNH3/", "geheim"},
	{"$6$ rounds low", "$6$rounds=1400$short$XQY.t06R.GTyvMmXC4tfzgEtZH8ep2g/gmZexJGxxWgf0pHZ0vYkQ0JTguvnWC6CYYNuHaili3ph6nF1pPDuy0", "geheim"},
	{"$5$", "$5$saltstring$5B8vYYiY.CVt1RlTTf8KbXBH3hsxY/GNooZaBBGWEc5", "Hello world!"},
	{"$5$ rounds", "$5$rounds=9000$rsalt$6wCpctKderndkZx7WILTYLVkAeifuCuz33Z0LJki2UD", "geheim"},
	// A setting below the minimum is raised to 1000, as glibc does and perl shows.
	{"$6$ rounds floor", "$6$rounds=1000$s$QqnZz8S/0z0aFGP6ajZoosVtMNUlt3lbjBJhxOey0TOKNRr6qN55ab28jMq3xnHV6SgQP29RlHMsEpuYFJc9l.", "kurz"},
}

// TestVerifyPasswordAgainstTheSystem: a hash from openssl must verify, and a
// different password must not.
func TestVerifyPasswordAgainstTheSystem(t *testing.T) {
	for _, c := range hashCases {
		ok, err := verifyPassword(c.hash, c.password)
		if err != nil {
			t.Errorf("%s: %v", c.scheme, err)
			continue
		}
		if !ok {
			t.Errorf("%s: correct password rejected", c.scheme)
		}
		ok, err = verifyPassword(c.hash, c.password+"x")
		if err != nil {
			t.Errorf("%s wrong password: %v", c.scheme, err)
			continue
		}
		if ok {
			t.Errorf("%s: wrong password accepted", c.scheme)
		}
	}
}

// TestVerifyPasswordNamesUnsupportedSchemes: yescrypt and bcrypt are not in this
// build, and "unsupported" must not be reported as "wrong password".
func TestVerifyPasswordNamesUnsupportedSchemes(t *testing.T) {
	for _, h := range []string{"$y$j9T$abcdef", "$2b$12$abcdef", "", "abcdefghijklm"} {
		if _, err := verifyPassword(h, "x"); err != ErrUnsupportedHash {
			t.Errorf("hash %q: error %v, want ErrUnsupportedHash", h, err)
		}
	}
}

// withShadow points the shadow database at a temporary file and makes a wrong
// password cost nothing.
func withShadow(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/shadow"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	oldFile, oldDelay := shadowFile, failDelay
	shadowFile, failDelay = path, 0
	t.Cleanup(func() { shadowFile, failDelay = oldFile, oldDelay })
}

// TestAuthenticateAcceptsTheRightPassword is the whole point: the password from
// /etc/shadow, checked in this process.
func TestAuthenticateAcceptsTheRightPassword(t *testing.T) {
	h := hashCases[0]
	withShadow(t, "martin:"+h.hash+":19000:0:99999:7:::\n")
	var errb bytes.Buffer
	if err := authenticate("martin", strings.NewReader(h.password+"\n"), &errb); err != nil {
		t.Fatalf("%v, stderr %q", err, errb.String())
	}
	if !strings.Contains(errb.String(), "Password: ") {
		t.Fatalf("no prompt, stderr %q", errb.String())
	}
}

// TestAuthenticateRejectsTheWrongPassword: and it says the same thing as an
// unknown name would.
func TestAuthenticateRejectsTheWrongPassword(t *testing.T) {
	h := hashCases[0]
	withShadow(t, "martin:"+h.hash+"::::::\n")
	var errb bytes.Buffer
	err := authenticate("martin", strings.NewReader("falsch\n"), &errb)
	if err != errIncorrect {
		t.Fatalf("error %v, want errIncorrect", err)
	}
}

// TestAuthenticateUnknownUserIsIndistinguishable: an unknown name must produce
// the same answer as a wrong password, including after the delay.
func TestAuthenticateUnknownUserIsIndistinguishable(t *testing.T) {
	withShadow(t, "root:x:19000:0:99999:7:::\n")
	start := time.Now()
	var errb bytes.Buffer
	err := authenticate("gibtsnicht", strings.NewReader("egal\n"), &errb)
	if err != errIncorrect {
		t.Fatalf("error %v, want errIncorrect", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("delay not overridden in the test")
	}
}

// TestAuthenticateRefusesLockedAndExpiredAccounts: the state of the account is
// reported, because that is the user's own business and not a stranger's guess.
func TestAuthenticateRefusesLockedAccount(t *testing.T) {
	withShadow(t, "martin:!$6$whatever::::::\n")
	var errb bytes.Buffer
	err := authenticate("martin", strings.NewReader("x\n"), &errb)
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("error %v, want a locked account", err)
	}
}

func TestReadShadowReportsExpiry(t *testing.T) {
	past := int(time.Now().Unix()/86400) - 10
	line := "martin:" + hashCases[0].hash + ":19000:0:99999:7::" + strconv.Itoa(past) + ":\n"
	withShadow(t, line)
	e, err := readShadow("martin")
	if err != nil {
		t.Fatal(err)
	}
	if e.State != "expired" {
		t.Fatalf("state %q, want expired", e.State)
	}
}
