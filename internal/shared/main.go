// Package shared holds the file helpers several small commands need, so that
// they are written once and not copied into each command (acmesh-mq1).

package shared

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func OpenInputs(names []string, stdin *os.File, stderr *os.File, cmd string) ([]io.Reader, int) {
	if len(names) == 0 {
		return []io.Reader{stdin}, 0
	}
	var readers []io.Reader
	for _, name := range names {
		if name == "-" {
			readers = append(readers, stdin)
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return nil, 1
		}
		readers = append(readers, f)
	}
	return readers, 0
}

func CloseInputs(readers []io.Reader) {
	for _, r := range readers {
		if c, ok := r.(io.Closer); ok && c != os.Stdin {
			c.Close()
		}
	}
}

func ReadAllLines(r io.Reader) ([]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := string(data)
	if text == "" {
		return nil, nil
	}
	text = strings.TrimSuffix(text, "\n")
	text = strings.TrimSuffix(text, "\r")
	return strings.Split(text, "\n"), nil
}
