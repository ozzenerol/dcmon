// Package cmd implements the command-line entry point and its configuration
package cmd

import (
	"log"
	"os"
	"strings"
	"unicode"
	"fmt"
	"golang.org/x/sys/unix"

	"github.com/peterh/liner"
	"dcmon/host"
)

const (
	DefaultSymbol				= "$"
	DefaultHistoryPath	= ".history"

	CommandExit					= "exit"
	CommandStatus				= "status"
	CommandHosts				= "hosts"

	FlagHostname			  = "-h"

	AnsiAltScreenOn  = "\033[?1049h"
  AnsiAltScreenOff = "\033[?1049l"
  AnsiCursorHide   = "\033[?25l"
  AnsiCursorShow   = "\033[?25h"
  AnsiCursorHome   = "\033[H"
  AnsiClearScreen  = "\033[2J"
)

type CmdConfig struct {
	Symbol      string
	SaveHistory bool
	HistoryPath string
	CtrlCAborts bool
}

func Execute(config CmdConfig) {
	if config.Symbol == "" {
		config.Symbol = DefaultSymbol
	}

	if config.SaveHistory && config.HistoryPath == "" {
		config.HistoryPath = DefaultHistoryPath
	}

	line := liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(config.CtrlCAborts)

	var file *os.File
	if config.SaveHistory {
		var err error
		path := config.HistoryPath
		file, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("could not open file %s: %v", config.HistoryPath, err)
		}

		defer file.Close()

		line.ReadHistory(file)
	}

	hosts, err := host.Load("config.json")
	if err != nil {
		log.Fatal(err)
	}

	for {
		command, err := line.Prompt(config.Symbol)
		if err == liner.ErrPromptAborted {
			continue
		}

		// EOF
		if err != nil {
			break
		}

		command = strings.TrimLeftFunc(command, unicode.IsSpace)

		if command == "" {
			continue
		}

		if command == CommandExit {
			break
		}

		if strings.HasPrefix(command, CommandStatus) {
			var args []string
			if after, ok := strings.CutPrefix(command, CommandStatus); ok {
				args = strings.Fields(after)
			}
			
			err := watch(func() error {
				if len(args) > 0 && args[0] == FlagHostname {
					return host.StatusByHostname(hosts, args[1])
				}

				return host.Status(hosts, args)
			})	
			
			if err != nil {
				fmt.Println(err)
			}
		}

		if command == CommandHosts {
			err = host.List(hosts)
			if err != nil {
				fmt.Println(err)
			}
		}

		line.AppendHistory(command)
	}

	if file != nil {
		file.Truncate(0)
		file.Seek(0, 0)
		line.WriteHistory(file)
	}
}

func watch(render func() error) error {
  fd := int(os.Stdin.Fd())
  old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
  if err != nil {
    return err
  }

  raw := *old
  raw.Lflag &^= unix.ICANON | unix.ECHO
  raw.Cc[unix.VMIN] = 1
  raw.Cc[unix.VTIME] = 0
  if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
    return err
  }
  defer unix.IoctlSetTermios(fd, unix.TCSETS, old)

  fmt.Print(AnsiAltScreenOn + AnsiCursorHide)
  defer fmt.Print(AnsiCursorShow + AnsiAltScreenOff)

	keys := make(chan byte) 
	go func() {
		buf := make([]byte, 1)
		for {
			if _, err := os.Stdin.Read(buf); err != nil {
				close(keys)
				return
			}
			keys <- buf[0]
		}
	}()

	for {
		fmt.Print(AnsiCursorHome + AnsiClearScreen)
		if err := render(); err != nil {
			fmt.Println(err)
		}
		fmt.Println("\nPress r to refresh, q to quit")
	
		wait:
			for {
				key, ok := <-keys
				switch {
				case !ok || key == 'q':
					return nil
				case key == 'r':
					break wait
				}
			}
	}
}
