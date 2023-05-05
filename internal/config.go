package internal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/andrewpillar/config"

	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	ConfigDir = "/etc/djinn"

	errNoConfig = errors.New("could not detect djinn configuration")

	serverConfig    = filepath.Join(ConfigDir, "/server.conf")
	uiConfig        = filepath.Join(ConfigDir, "/ui.conf")
	apiConfig       = filepath.Join(ConfigDir, "/api.conf")
	workerConfig    = filepath.Join(ConfigDir, "/worker.conf")
	curatorConfig   = filepath.Join(ConfigDir, "/curator.conf")
	consumerConfig  = filepath.Join(ConfigDir, "/consumer.conf")
	schedulerConfig = filepath.Join(ConfigDir, "/scheduler.conf")

	configFiles = [...]string{
		serverConfig,
		uiConfig,
		apiConfig,
		workerConfig,
		curatorConfig,
		consumerConfig,
		schedulerConfig,
	}
)

type ConfigError struct {
	Dir string
	Err error
}

func (e *ConfigError) Error() string {
	return e.Dir + ": " + e.Err.Error()
}

func DetectConfig(names ...string) (string, error) {
	set := make(map[string]struct{})

	for _, name := range names {
		set[name] = struct{}{}
	}

	var err error

	for _, name := range configFiles {
		_, err = os.Stat(name)

		if err != nil {
			continue
		}

		if len(names) > 0 {
			if _, ok := set[filepath.Base(name)]; !ok {
				continue
			}
		}
		return name, nil
	}
	return "", &ConfigError{
		Dir: ConfigDir,
		Err: errNoConfig,
	}
}

func DecodeConfig(v any, name string) error {
	opts := []config.Option{
		config.Includes,
		config.Envvars,
	}

	if err := config.DecodeFile(v, name, opts...); err != nil {
		return err
	}
	return nil
}

func DetectAndConnectDatabase(ctx context.Context) (*pgxpool.Pool, error) {
	name, err := DetectConfig("server.conf", "api.conf", "ui.conf", "worker.conf")

	if err != nil {
		return nil, err
	}

	var cfg struct {
		Database struct {
			Addr string
			Name string

			Username string
			Password string
		}
	}

	if err := DecodeConfig(&cfg, name); err != nil {
		return nil, err
	}

	host, port, err := net.SplitHostPort(cfg.Database.Addr)

	if err != nil {
		return nil, err
	}

	dsnfmt := "host=%s port=%s dbname=%s user=%s password=%s"
	dsn := fmt.Sprintf(dsnfmt, host, port, cfg.Database.Name, cfg.Database.Username, cfg.Database.Password)

	pool, err := pgxpool.Connect(ctx, dsn)

	if err != nil {
		return nil, err
	}
	return pool, nil
}
