package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/chouandy/goex/cryptoex"
	"github.com/chouandy/goex/dotenvex"
)

// EncryptCommand the command struct
type EncryptCommand struct {
	File     string
	Password string
}

// Synopsis the synopsis of command
func (c *EncryptCommand) Synopsis() string {
	return "Encrypt file"
}

// Help the help of command
func (c *EncryptCommand) Help() string {
	helpText := `
Usage: filecryptor enc
	Encrypt file

Options:
  --file         Target file to be encrypted.
  --password     The password for encrypt. It can be ENV["SECRETS_PASSWORD"]
`
	return strings.TrimSpace(helpText)
}

// Run the main execution of command
func (c *EncryptCommand) Run(args []string) int {
	// Init flag
	f := flag.NewFlagSet("enc", flag.ContinueOnError)
	f.StringVar(&c.File, "file", "", "file")
	f.StringVar(&c.Password, "password", "", "password")
	if err := f.Parse(args); err != nil {
		fmt.Println(err)
		return 1
	}

	// Get password from env or awa parameter store
	if len(c.Password) == 0 {
		c.Password = dotenvex.GetSecretsPassword()
	}

	// Validate Options
	fmt.Print("Validate Options...")
	if err := c.ValidateOptions(); err != nil {
		fmt.Println(err)
		return 1
	}
	fmt.Println("done")

	// Encrypt File
	fmt.Printf("Encrypt File...")
	src := c.File
	dst := c.File + ".enc"
	if err := cryptoex.FileEncrypter(src, dst, []byte(c.Password)); err != nil {
		fmt.Println(err)
		return 1
	}
	fmt.Println("done")

	return 0
}

// ValidateOptions validate options
func (c *EncryptCommand) ValidateOptions() error {
	if len(c.File) == 0 {
		return errors.New("file can't be blank")
	}
	if len(c.Password) == 0 {
		return errors.New("password can't be blank")
	}

	return nil
}
