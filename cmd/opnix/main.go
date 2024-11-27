package main

import (
    "fmt"
    "log"
    "os"
)

type command interface {
    Name() string
    Init([]string) error
    Run() error
}

func main() {
    cmds := []command{
        newSecretCommand(),
        newTokenCommand(),
    }

    if len(os.Args) < 2 {
        printUsage(cmds)
        os.Exit(1)
    }

    subcommand := os.Args[1]

    for _, cmd := range cmds {
        if cmd.Name() == subcommand {
            if err := cmd.Init(os.Args[2:]); err != nil {
                log.Fatalf("Failed to initialize %s: %v", cmd.Name(), err)
            }
            if err := cmd.Run(); err != nil {
                log.Fatalf("Failed to run %s: %v", cmd.Name(), err)
            }
            return
        }
    }

    fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcommand)
    printUsage(cmds)
    os.Exit(1)
}

func printUsage(cmds []command) {
    fmt.Fprintf(os.Stderr, "Usage: opnix <command> [options]\n\n")
    fmt.Fprintf(os.Stderr, "Available commands:\n")
    fmt.Fprintf(os.Stderr, "  secret    Manage and retrieve secrets from 1Password\n")
    fmt.Fprintf(os.Stderr, "  token     Manage the 1Password service account token\n\n")
    fmt.Fprintf(os.Stderr, "Use 'opnix <command> -h' for command-specific help\n")
}
