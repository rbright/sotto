package cli

import (
	"errors"
	"fmt"
	"strings"
)

type Command string

const (
	CommandToggle  Command = "toggle"
	CommandStop    Command = "stop"
	CommandCancel  Command = "cancel"
	CommandStatus  Command = "status"
	CommandDevices Command = "devices"
	CommandDoctor  Command = "doctor"
	CommandVersion Command = "version"
	CommandHelp    Command = "help"
)

var validCommands = map[Command]struct{}{
	CommandToggle:  {},
	CommandStop:    {},
	CommandCancel:  {},
	CommandStatus:  {},
	CommandDevices: {},
	CommandDoctor:  {},
	CommandVersion: {},
	CommandHelp:    {},
}

type Parsed struct {
	Command    Command
	ConfigPath string
	ShowHelp   bool
}

func Parse(args []string) (Parsed, error) {
	parsed := Parsed{Command: CommandHelp, ShowHelp: true}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch arg {
		case "-h", "--help":
			parsed.ShowHelp = true
			parsed.Command = CommandHelp
		case "--version":
			parsed.ShowHelp = false
			parsed.Command = CommandVersion
		case "--config":
			i++
			if i >= len(args) {
				return Parsed{}, errors.New("--config requires a path")
			}
			parsed.ConfigPath = args[i]
		default:
			if strings.HasPrefix(arg, "-") {
				return Parsed{}, fmt.Errorf("unknown flag: %s", arg)
			}

			cmd := Command(arg)
			if _, ok := validCommands[cmd]; !ok {
				return Parsed{}, fmt.Errorf("unknown command: %s", arg)
			}

			parsed.Command = cmd
			parsed.ShowHelp = cmd == CommandHelp
			if i != len(args)-1 {
				return Parsed{}, fmt.Errorf("unexpected arguments after command %q", arg)
			}
		}
	}

	return parsed, nil
}

func HelpText(binaryName string) string {
	return fmt.Sprintf(`Usage:
  %[1]s [--config PATH] <command>

Commands:
  toggle    Start recording or stop+commit when already recording
  stop      Stop active recording and commit transcript
  cancel    Cancel active recording and discard transcript
  status    Print current state
  devices   List available input devices
  doctor    Run configuration and environment checks
  version   Print version information
  help      Show this help

Flags:
  --config PATH   Config file path (default: $XDG_CONFIG_HOME/sotto/config.conf)
  -h, --help      Show help
  --version       Show version
`, binaryName)
}
