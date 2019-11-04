package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/rand"
	"strings"
	"time"
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
Usage: filecryptor pass
	Random hex string

Options:
	-n           The size of hex.
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

	// New random hex
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, c.N)
	r.Read(b)

	// Hex encode
	enc := make([]byte, len(b)*2)
	hex.Encode(enc, b)

	fmt.Println(string(enc))

	return 0
}
