package main

import "github.com/urfave/cli/v3"

func newApp() *cli.Command {
	return &cli.Command{
		Name:  "jtp",
		Usage: "Enhanced Jujutsu workspace management",
		Description: "jtp simplifies Jujutsu workspace creation with automatic bookmark tracking, " +
			"project-specific setup hooks, and convenient defaults.",
		Version:                         version,
		EnableShellCompletion:           true,
		ConfigureShellCompletionCommand: configureCompletionCommand,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "version",
				Usage: "Show version information",
			},
		},
		Commands: []*cli.Command{
			NewAddCommand(),
			NewListCommand(),
			NewRemoveCommand(),
			NewInitCommand(),
			NewCdCommand(),
			NewExecCommand(),
			// Built-in completion is automatically provided by urfave/cli
			NewHookCommand(),
			NewShellInitCommand(),
		},
	}
}
