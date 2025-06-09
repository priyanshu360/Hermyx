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
  up        Start the Hermyx reverse proxy
	down 			Close the Hermyx reverse proxy
  help      Show help for a command

Run 'hermyx help <command>' for details on a specific command.`)
}

func printUpHelp() {
	fmt.Println(`Usage:
  hermyx up [--config <path>]

Options:
  --config   Path to Hermyx config YAML file (default: ./hermyx.config.yaml)`)
}

func printDownHelp() {
	fmt.Println(`Usage:
  hermyx down [--config <path>]

Options:
  --config   Path to Hermyx config YAML file (default: ./hermyx.config.yaml)`)
}

func main() {
	if len(os.Args) < 2 {
		printRootHelp()
		os.Exit(1)
	}

	switch os.Args[1] {

	case "up":
		runCmd := flag.NewFlagSet("up", flag.ExitOnError)
		configPath := runCmd.String("config", "hermyx.config.yaml", "Path to configuration YAML file")

		// Parse flags after "up"
		if err := runCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
			os.Exit(1)
		}

		absPath, err := filepath.Abs(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to resolve config path: %v\n", err)
			os.Exit(1)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Config file not found: %s\n", absPath)
			os.Exit(1)
		}

		proxyEngine := engine.InstantiateHermyxEngine(absPath)
		proxyEngine.Run()

	case "down":
		runCmd := flag.NewFlagSet("up", flag.ExitOnError)
		configPath := runCmd.String("config", "hermyx.config.yaml", "Path to configuration YAML file")

		// Parse flags after "up"
		if err := runCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
			os.Exit(1)
		}

		absPath, err := filepath.Abs(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to resolve config path: %v\n", err)
			os.Exit(1)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Config file not found: %s\n", absPath)
			os.Exit(1)
		}

		err = engine.KillHermyx(absPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to kill the hermyx server at %s: %v\n", *configPath, err)
			os.Exit(1)
		}
		fmt.Printf("Shut down hermyx server at %s \n", *configPath)

	case "help":
		if len(os.Args) == 2 {
			printRootHelp()
		} else {
			switch os.Args[2] {
			case "up":
				printUpHelp()
			case "down":
				printDownHelp()
			default:
				fmt.Printf("Unknown help topic: %s\n", os.Args[2])
				printRootHelp()
				os.Exit(1)
			}
		}

	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		printRootHelp()
		os.Exit(1)
	}
}
