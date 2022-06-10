package internal

import (
	"errors"
	"os"

	"github.com/andrewpillar/config"
)

var (
	ConfigDir = "/etc/djinn"

	errNoConfig = errors.New("could not detect djinn configuration")

	configFiles = []string{
		ConfigDir + "/server.conf",
		ConfigDir + "/ui.conf",
		ConfigDir + "/api.conf",
		ConfigDir + "/worker.conf",
		ConfigDir + "/curator.conf",
		ConfigDir + "/consumer.conf",
		ConfigDir + "/scheduler.conf",
	}
)

type ConfigError struct {
	Dir string
	Err error
}

func (e *ConfigError) Error() string {
	return e.Dir + ": " + e.Err.Error()
}

func DetectConfig() (string, error) {
	var err error

	for _, name := range configFiles {
		_, err = os.Stat(name)

		if err != nil {
			continue
		}
		return name, nil
	}
	return "", &ConfigError{
		Dir: ConfigDir,
		Err: errNoConfig,
	}
}

func DecodeConfig(v interface{}, name string) error {
	opts := []config.Option{
		config.Includes,
		config.Envvars,
	}

	if err := config.DecodeFile(v, name, opts...); err != nil {
		return err
	}
	return nil
}
