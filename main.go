package main

import (
	"log"
	"os"

	"github.com/mitchellh/cli"
)

func main() {
	// New cli
	c := cli.NewCLI("filecryptor", version)
	// Set args
	c.Args = os.Args[1:]
	// Set commands
	c.Commands = map[string]cli.CommandFactory{
		"enc": func() (cli.Command, error) {
			return &EncryptCommand{}, nil
		},
		"dec": func() (cli.Command, error) {
			return &DecryptCommand{}, nil
		},
	}
	// Run cli
	exitStatus, err := c.Run()
	if err != nil {
		log.Println(err)
	}

	os.Exit(exitStatus)
}
