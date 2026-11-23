//go:build !windows

package login

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// shadowFile is where the hashes live. It is readable by root only, which is
// the same rule the passwd file already follows for login.
var shadowFile = "/etc/shadow"

// shadowEntry is the part of a /etc/shadow line this command needs.
type shadowEntry struct {
	Hash    string
	Expires int    // day number since 1970, -1 when the field is empty
	State   string // "ok", "locked", "expired", "no-password"
}

// readShadow finds the record for name and says in plain words what state the
// account is in. A missing or unreadable shadow file is an error, never a free
// pass.
func readShadow(name string) (shadowEntry, error) {
	f, err := openShadow()
	if err != nil {
		return shadowEntry{}, err
	}
	defer f.Close()
	sc := newScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) < 2 || fields[0] != name {
			continue
		}
		e := shadowEntry{Hash: fields[1], Expires: -1}
		if len(fields) > 7 && fields[7] != "" {
			if days, err := strconv.Atoi(fields[7]); err == nil {
				e.Expires = days
			}
		}
		switch {
		case e.Hash == "":
			e.State = "no-password"
		case strings.HasPrefix(e.Hash, "!") || strings.HasPrefix(e.Hash, "*"):
			e.State = "locked"
		case e.Expires >= 0 && int(time.Now().Unix()/86400) > e.Expires:
			e.State = "expired"
		default:
			e.State = "ok"
		}
		return e, nil
	}
	if err := sc.Err(); err != nil {
		return shadowEntry{}, err
	}
	return shadowEntry{}, fmt.Errorf("no shadow entry for %q", name)
}
