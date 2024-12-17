package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
)

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func bye(vars ...any) {
	fmt.Fprintln(os.Stderr, vars...)
	os.Exit(1)
}

func runCommand(command []string, cwd string) *exec.Cmd {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = cwd

	err := cmd.Start()
	if err != nil {
		bye("Error starting command:", err)
	}

	return cmd
}

func main() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, unix.SIGQUIT)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		bye(err)
	}
	defer watcher.Close()

	filenameRegex, _ := regexp.Compile(`.*`)
	dir := "."
	cwd := "."
	command := []string{}
	var currentProcess *exec.Cmd

	for i := 0; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "-f") {
			if i == len(os.Args)-1 {
				bye("Expected filename regex to follow -f flag")
			}
			filenameRegex, err = regexp.Compile(os.Args[i+1])
			if err != nil {
				bye(err)
			}
			i++
		} else if strings.HasPrefix(arg, "-d") {
			if i == len(os.Args)-1 {
				bye("Expected directory to follow -d flag")
			}
			dir = os.Args[i+1]
			i++
		} else if strings.HasPrefix(arg, "-w") {
			if i == len(os.Args)-1 {
				bye("Expected directory to follow -w flag")
			}
			cwd = os.Args[i+1]
			i++
		} else if arg == "--" {
			if i == len(os.Args)-1 {
				bye("Expected command to run to follow --")
			}
			i++
			command = os.Args[i:]
		}
	}

	if len(command) == 0 {
		bye("Usage: pacz [-d <directory>] [-f <filename-regex>] [-w <command-cwd>] -- <command>")
	}

	currentProcess = runCommand(command, cwd)

	err = watcher.Add(dir)
	if err != nil {
		bye(err)
	}

	clearScreen()

	for {
		select {
		case event := <-watcher.Events:
			if filenameRegex.MatchString(event.Name) &&
				(event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Remove == fsnotify.Remove ||
					event.Op&fsnotify.Write == fsnotify.Write) {

				if currentProcess != nil && currentProcess.Process != nil {
					currentProcess.Process.Kill()
				}

				clearScreen()

				currentProcess = runCommand(command, cwd)
			}
		case err := <-watcher.Errors:
			bye(err)
		case <-signalChan:
			clearScreen()
			os.Exit(0)
		}
	}
}
