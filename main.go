package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"

	"github.com/hay-kot/ghdb/app/clients"
	"github.com/hay-kot/ghdb/app/commands"
	"github.com/hay-kot/ghdb/app/commands/gdb"
)

var (
	// Build information. Populated at build-time via -ldflags flag.
	version = "dev"
	commit  = "HEAD"
	date    = "now"
)

func build() string {
	short := commit
	if len(commit) > 7 {
		short = commit[:7]
	}

	return fmt.Sprintf("%s (%s) %s", version, short, date)
}

func home() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	return home
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	ctrl := &commands.Controller{}

	app := &cli.App{
		Name:    "ghdb",
		Usage:   "Command Line TUI for quick access to multiple Github Orgs",
		Version: build(),
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:        "cache-dir",
				Usage:       "cache directory",
				Value:       filepath.Join(home(), ".config", "ghdb"),
				EnvVars:     []string{"GHDB_CACHE_DIR"},
				Destination: &ctrl.CacheDir,
			},
			&cli.StringFlag{
				Name:  "log-level",
				Usage: "log level (debug, info, warn, error, fatal, panic)",
				Value: "panic",
			},
			&cli.PathFlag{
				Name:    "config",
				Usage:   "config file path",
				Value:   filepath.Join(home(), ".config", "ghdb", "config.yml"),
				EnvVars: []string{"GHDB_CONFIG"},
			},
		},
		Before: func(ctx *cli.Context) error {
			configPath := ctx.String("config")

			// Load config
			f, err := os.Open(configPath)
			if err != nil {
				return fmt.Errorf("failed to open config file: %w", err)
			}

			defer func() { _ = f.Close() }()

			conf, err := gdb.ReadConfig(f)
			if err != nil {
				return fmt.Errorf("failed to read config file: %w", err)
			}

			cacheFile := filepath.Join(ctrl.CacheDir, "cache.json")

			ctrl.GitHub = clients.NewGitHub()
			ctrl.GitDB = gdb.New(cacheFile, conf)

			switch ctx.String("log-level") {
			case "debug":
				log.Level(zerolog.DebugLevel)
			case "info":
				log.Level(zerolog.InfoLevel)
			case "warn":
				log.Level(zerolog.WarnLevel)
			case "error":
				log.Level(zerolog.ErrorLevel)
			case "fatal":
				log.Level(zerolog.FatalLevel)
			default:
				log.Level(zerolog.PanicLevel)
			}

			// Ensure CacheDir exists
			if _, err := os.Stat(ctrl.CacheDir); os.IsNotExist(err) {
				if err := os.MkdirAll(ctrl.CacheDir, 0755); err != nil {
					return err
				}
			}

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "sync",
				Usage: "sync all github org, user, and PR data",
				Action: ctrl.Sync,
			},
			{
				Name:   "find",
				Usage:  "search cache",
				Action: ctrl.Find,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("failed to run ghdb")
	}
}
