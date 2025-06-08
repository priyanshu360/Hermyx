package main

import (
	"flag"
	"fmt"
	"hermyx/pkg/engine"
	"os"
	"path/filepath"
)

func printRootHelp() {
	fmt.Println(`hermyx - blazing fast reverse proxy with smart caching

Usage:
  hermyx <command> [options]

Available Commands:
  run       Start the Hermyx reverse proxy
  help      Show help for a command

Run 'hermyx help <command>' for details on a specific command.`)
}

func printRunHelp() {
	fmt.Println(`Usage:
  hermyx up [--config <path>]

Options:
  --config    Path to hermyx config YAML (default: ./hermyx.config.yaml)`)
}

func main() {
	if len(os.Args) < 2 {
		printRootHelp()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "up":
		runCmd := flag.NewFlagSet("up", flag.ExitOnError)
		configPath := runCmd.String("config", "hermyx.config.yaml", "Path to configuration YAML file")

		if err := runCmd.Parse(os.Args[2:]); err != nil {
			fmt.Printf("Failed to parse flags: %v\n", err)
			os.Exit(1)
		}

		absPath, err := filepath.Abs(*configPath)
		if err != nil {
			fmt.Printf("Unable to resolve config path: %v\n", err)
			os.Exit(1)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			fmt.Printf("Config file not found: %s\n", absPath)
			os.Exit(1)
		}

		engine := engine.InstantiateHermyxEngine(absPath)
		engine.Run()

	case "help":
		if len(os.Args) == 2 {
			printRootHelp()
		} else {
			switch os.Args[2] {
			case "up":
				printRunHelp()
			default:
				fmt.Printf("Unknown help topic: %s\n", os.Args[2])
			}
		}

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printRootHelp()
		os.Exit(1)
	}
}
