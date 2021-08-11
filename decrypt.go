package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chouandy/go-sdk/crypto"
	"github.com/chouandy/go-sdk/dotenv"
)

// DecryptCommand the command struct
type DecryptCommand struct {
	File     string
	Password string
	PSName   string
	PSRegion string
}

// Synopsis the synopsis of command
func (c *DecryptCommand) Synopsis() string {
	return "Decrypt file"
}

// Help the help of command
func (c *DecryptCommand) Help() string {
	helpText := `
Usage: filecryptor dec
	Decrypt file

Options:
  --file         Target file to be decrypted.
  --password     The password for decrypt. It can be ENV["SECRETS_PASSWORD"]
  --ps-name      The parameter store name for decrypt. It can be ENV["SECRETS_PASSWORD_PS_NAME"]
  --ps-region    The parameter store region for decrypt. It can be ENV["SECRETS_PASSWORD_PS_REGION"]
`
	return strings.TrimSpace(helpText)
}

// Run the main execution of command
func (c *DecryptCommand) Run(args []string) int {
	// Init flag
	f := flag.NewFlagSet("dec", flag.ContinueOnError)
	f.StringVar(&c.File, "file", "", "file")
	f.StringVar(&c.Password, "password", "", "password")
	f.StringVar(&c.PSName, "ps-name", "", "ps-name")
	f.StringVar(&c.PSRegion, "ps-region", "us-east-1", "ps-region")
	if err := f.Parse(args); err != nil {
		fmt.Println(err)
		return 1
	}

	if len(c.PSName) > 0 {
		os.Setenv("SECRETS_PASSWORD_PS_NAME", c.PSName)
		os.Setenv("SECRETS_PASSWORD_PS_REGION", c.PSRegion)
	}

	// Get password from env or awa parameter store
	if len(c.Password) == 0 {
		c.Password = dotenv.GetSecretsPassword()
	}

	// Validate Options
	fmt.Print("Validate Options...")
	if err := c.ValidateOptions(); err != nil {
		fmt.Println(err)
		return 1
	}
	fmt.Println("done")

	// Decrypt File
	fmt.Printf("Decrypt File...")
	src := c.File
	dst := strings.Replace(c.File, ".enc", "", -1)
	if err := crypto.FileDecrypter(src, dst, []byte(c.Password)); err != nil {
		fmt.Println(err.Error())
		return 1
	}
	fmt.Println("done")

	return 0
}

// ValidateOptions validate options
func (c *DecryptCommand) ValidateOptions() error {
	if len(c.File) == 0 {
		return errors.New("file can't be blank")
	}
	if len(c.Password) == 0 {
		return errors.New("password can't be blank")
	}

	return nil
}
