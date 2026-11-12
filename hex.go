package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"strings"
)

// HexCommand the command struct
type HexCommand struct {
	N int
}

// Synopsis the synopsis of command
func (c *HexCommand) Synopsis() string {
	return "Random hex string"
}

// Help the help of command
func (c *HexCommand) Help() string {
	helpText := `
Usage: filecryptor hex
	Random hex string from a cryptographically secure source, fit for passwords

Options:
	-n           Number of random bytes, printed as twice as many hex characters. Default 16.
`
	return strings.TrimSpace(helpText)
}

// Run the main execution of command
func (c *HexCommand) Run(args []string) int {
	// Init flag
	f := flag.NewFlagSet("hex", flag.ContinueOnError)
	f.IntVar(&c.N, "n", 16, "n")
	if err := f.Parse(args); err != nil {
		fmt.Println(err)
		return 1
	}
	if c.N < 1 {
		fmt.Println("n must be at least 1")
		return 1
	}

	b := make([]byte, c.N)
	if _, err := rand.Read(b); err != nil {
		fmt.Println(err)
		return 1
	}
	fmt.Println(hex.EncodeToString(b))

	return 0
}
