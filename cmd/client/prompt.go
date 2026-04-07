package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

func promptForMissingConnectionValues(server *string, token *string) error {
	if strings.TrimSpace(*server) != "" && strings.TrimSpace(*token) != "" {
		return nil
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	if strings.TrimSpace(*server) == "" {
		value, err := promptLine(reader, "Enter tunnel server address (ip:port): ")
		if err != nil {
			return err
		}
		*server = value
	}
	if strings.TrimSpace(*token) == "" {
		value, err := promptLine(reader, "Enter auth token: ")
		if err != nil {
			return err
		}
		*token = value
	}

	return nil
}

func promptLine(reader *bufio.Reader, label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}
