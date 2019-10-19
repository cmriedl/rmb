package main

// SPDX-License-Identifier: GPL-3.0-only
//
// Copyright (C) 2019 cmr@informatik.wtf
// This file is part of rmb.
//
// rmb is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3.
//
// rmb is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with rmb. If not, see <https://www.gnu.org/licenses/>.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const usage = `usage: rmb <conf>
       rmb <conf> page 
       rmb -h

'rmb <conf>'		Compile the website described in the YAML conf file
'rmb <conf> page' 	Create an empty index or empty post via cli prompts

Copyright (C) 2019  cmr@informatik.wtf

This program comes with ABSOLUTELY NO WARRANTY. This is free software,
and you are welcome to redistribute it under certain conditions; see
the provided COPYING file.`

func console(w io.Writer, s string, vargs ...interface{}) {
	if !strings.HasPrefix(s, "rmb: ") {
		s = "rmb: " + s
	}
	switch {
	case len(vargs) == 0:
		fmt.Fprintf(w, s)
	default:
		fmt.Fprintf(w, s, vargs...)
	}
}

func stdout(s string, vargs ...interface{}) {
	console(os.Stdout, s, vargs...)
}

func stderr(s string, vargs ...interface{}) {
	console(os.Stderr, s, vargs...)
}

func kill(err error, s string, vargs ...interface{}) {
	if perr(err, s, vargs...) {
		os.Exit(1)
	}
	return
}

func perr(err error, s string, vargs ...interface{}) (ok bool) {
	if err != nil {
		stderr("%+v\n", err)
		if s != "" {
			stderr(s, vargs...)
		}
		ok = true
	}
	return
}

func unixfy(f string) (s string) {
	var x []rune
	for _, c := range f {
		if c >= 'A' && c <= 'Z' {
			// Lowercase
			c += 0x20
		} else if c == ' ' {
			// Replace space with underscore
			c = '_'
		} else if !(c >= 'a' && c <= 'z' || c == '_' ||
			c >= '0' && c <= '9') {
			// Drop any non-alphanumeric or underscore character
			continue
		}
		x = append(x, c)
	}
	s = string(x)
	return
}

func dircpy(from string, dest string) (err error) {
	files, err := filepath.Glob(filepath.Join(from, "*"))
	if perr(err, "") {
		return
	}

	var cmderr bytes.Buffer
	for _, f := range files {
		cmd := exec.Command("cp", f, dest)
		cmd.Stderr = &cmderr
		cmderr.Reset()

		err := cmd.Run()
		if err != nil {
			stderr("%+s\n", cmderr)
			break
		}
	}
	return
}

func tostring(b []byte) (s string) {
	s = string(b)
	return
}
