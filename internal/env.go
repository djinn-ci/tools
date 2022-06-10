package internal

import (
	"bufio"
	"os"
	"strings"
)

var envfile = "/etc/default/djinn"

func LoadEnv() error {
	f, err := os.Open(envfile)

	if err != nil {
		return err
	}

	defer f.Close()

	sc := bufio.NewScanner(f)

	for sc.Scan() {
		line := sc.Text()

		if len(line) == 0 {
			continue
		}

		if line[0] == '#' {
			continue
		}

		parts := strings.SplitN(line, "=", 2)

		os.Setenv(parts[0], parts[1])
	}

	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}
