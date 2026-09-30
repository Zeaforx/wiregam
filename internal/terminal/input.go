package terminal

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// ReadInput reads a line of regular input from the terminal.
func ReadInput(prompt string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(input), nil
}

// ReadSecretInput reads input without echoing it to the terminal.
func ReadSecretInput(prompt string) (string, error) {
	fmt.Print(prompt)

	bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
