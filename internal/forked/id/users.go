// Copyright 2017-2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !plan9

//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/id/users.go.
// Local changes are listed below this note; the rest of this file is the
// upstream source and keeps its copyright.

package id

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Users manages a user database, typically loaded from /etc/passwd
type Users struct {
	uidToUser map[int]string
	userToUID map[string]int
	userToGID map[int]int
}

// GetUID returns the UID of a username
func (u *Users) GetUID(name string) (int, error) {
	if v, ok := u.userToUID[name]; ok {
		return v, nil
	}
	return -1, fmt.Errorf("unknown user name: %s", name)
}

// GetGID returns the primary GID of a UID
func (u *Users) GetGID(uid int) (int, error) {
	if v, ok := u.userToGID[uid]; ok {
		return v, nil
	}
	return -1, fmt.Errorf("unknown uid: %d", uid)
}

// GetUser returns the username of a UID
func (u *Users) GetUser(uid int) (string, error) {
	if v, ok := u.uidToUser[uid]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown uid: %d", uid)
}

// NewUsers is a factory for Users.  file is the file to read the database from.
func NewUsers(file string) (u *Users, e error) {
	u = &Users{}
	u.uidToUser = make(map[int]string)
	u.userToUID = make(map[string]int)
	u.userToGID = make(map[int]int)

	passwdFile, err := os.Open(file)
	if err != nil {
		// Local change: a system without an account database is not an error.
		// The caller then prints the numeric uid, as GNU id does.
		if os.IsNotExist(err) {
			return u, nil
		}
		return u, err
	}

	// Read from passwdFile for the users name
	var passwdInfo []string

	passwdScanner := bufio.NewScanner(passwdFile)

	for passwdScanner.Scan() {
		txt := passwdScanner.Text()
		if len(txt) == 0 || txt[0] == '#' { // skip empty lines and comments
			continue
		}
		passwdInfo = strings.Split(txt, ":")
		userNum, err := strconv.Atoi(passwdInfo[2])
		if err != nil {
			return nil, err
		}
		groupNum, err := strconv.Atoi(passwdInfo[3])
		if err != nil {
			return nil, err
		}

		u.uidToUser[userNum] = passwdInfo[0]
		u.userToUID[passwdInfo[0]] = userNum
		u.userToGID[userNum] = groupNum
	}

	return
}
