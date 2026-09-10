package cli

import (
	"fmt"
	"os"
	"strings"

	"hasucalc/version"
)

// Subcommands supported in HasuCalc CLI
var subcommands = map[string]bool{
	"convert": true,
	"info":    true,
	"get":     true,
	"eval":    true,
	"chart":   true,
	"set":     true,
	"batch":   true,
	"mcp":     true,
	"help":    true,
}

// IsSubcommand checks whether the provided string is a recognized headless subcommand.
func IsSubcommand(name string) bool {
	return subcommands[strings.ToLower(strings.TrimSpace(name))]
}

// Run dispatches the headless CLI command and returns the process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		PrintUsage()
		return 0
	}

	cmd := strings.ToLower(strings.TrimSpace(args[0]))
	cmdArgs := args[1:]

	// Check if user requested help on a subcommand (e.g. `hasucalc convert --help` or `-h`)
	if len(cmdArgs) > 0 && (cmdArgs[0] == "--help" || cmdArgs[0] == "-h") {
		PrintSubcommandHelp(cmd)
		return 0
	}

	switch cmd {
	case "convert":
		return RunConvert(cmdArgs)
	case "info":
		return RunInfo(cmdArgs)
	case "get":
		return RunGet(cmdArgs)
	case "eval":
		return RunEval(cmdArgs)
	case "chart":
		return RunChart(cmdArgs)
	case "set":
		return RunSet(cmdArgs)
	case "batch":
		return RunBatch(cmdArgs)
	case "mcp":
		return RunMCP(cmdArgs)
	case "help":
		if len(cmdArgs) > 0 {
			PrintSubcommandHelp(cmdArgs[0])
		} else {
			PrintUsage()
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown subcommand '%s'\n", cmd)
		PrintUsage()
		return 2
	}
}

// PrintSubcommandHelp prints specific help for a known subcommand.
func PrintSubcommandHelp(cmd string) {
	switch strings.ToLower(strings.TrimSpace(cmd)) {
	case "convert":
		PrintConvertHelp()
	case "info":
		PrintInfoHelp()
	case "get":
		PrintGetHelp()
	case "eval":
		PrintEvalHelp()
	case "chart":
		PrintChartHelp()
	case "set":
		PrintSetHelp()
	case "batch":
		PrintBatchHelp()
	case "mcp":
		PrintMCPHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand '%s'\n", cmd)
		PrintUsage()
	}
}

// PrintUsage prints general help for the headless CLI.
func PrintUsage() {
	msg := fmt.Sprintf(`HasuCalc %s — Headless CLI & AI Agent Toolkit

Usage:
  hasucalc <command> [arguments] [flags]

Available Commands:
  convert    Convert tabular data between formats (file-to-file or pipeline)
  info       Extract structural and sheet metadata from a workbook
  get        Extract cell data (sparse JSON, Markdown tables, CSV, or values)
  eval       Evaluate formulas immediately (standalone or in workbook context)
  chart      Render HD PNG charts from sheet data without terminal screen
  set        Mutate cell values, formulas, or formats with atomic persistence
  batch      Execute transactional mutation actions from JSON script or stdin
  mcp        Launch the Model Context Protocol (MCP) stdio server
  help       Help about any command

Run 'hasucalc <command> --help' for details on each subcommand.
For interactive TUI mode, run 'hasucalc [file]' or 'hasucalc --demo'.
`, version.Version)
	fmt.Print(msg)
}
