package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

// Set by -ldflags at build time. Falls back to embedded module version when
// installed via `go install` without ldflags.
var version = "dev"

func getVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func main() {
	args, useJSON := argsAfterFilters()
	useJSONMode = useJSON

	if len(args) < 2 {
		if useJSONMode {
			writeJSONResult(nil, false, "no command given (see: wiki-engine help)")
		} else {
			usage(os.Stderr)
		}
		os.Exit(1)
	}

	cmd := args[1]

	// -h/--help after a command is a help request, not an argument.
	afterTerminator := false
	for _, a := range args[2:] {
		if a == "--" {
			afterTerminator = true
			continue
		}
		if !afterTerminator && (a == "-h" || a == "--help") {
			usage(os.Stdout)
			return
		}
	}

	switch cmd {
	case "init":
		if err := validateCommandArgs("init", args[2:]); err != nil {
			fatal(err)
		}
		runInit(args, useJSON)
	case "sync-prompts":
		if err := validateCommandArgs("sync-prompts", args[2:]); err != nil {
			fatal(err)
		}
		runSyncPrompts(useJSON)
	case "schema":
		if err := validateCommandArgs("schema", args[2:]); err != nil {
			fatal(err)
		}
		runSchema(args, useJSON)
	case "version":
		if err := validateCommandArgs("version", args[2:]); err != nil {
			fatal(err)
		}
		if useJSON {
			writeJSON(getVersion())
		} else {
			fmt.Println(getVersion())
		}
	case "upgrade":
		if err := validateCommandArgs("upgrade", args[2:]); err != nil {
			fatal(err)
		}
		runUpgrade(args, useJSON)
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		cfg, eng := loadEngine()
		runEngine(cmd, cfg, eng, args, useJSON)
	}
}
