package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// EncryptCommand the command struct
type EncryptCommand struct {
	File     string
	Password string
	PSName   string
	PSRegion string
	Yes      bool
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
  --ps-name      The parameter store name for encrypt. It can be ENV["SECRETS_PASSWORD_PS_NAME"]
  --ps-region    The parameter store region for encrypt. It can be ENV["SECRETS_PASSWORD_PS_REGION"]
  --yes          Write without asking. The key changes against the existing {FILE}.enc are still printed.

Before writing, enc decrypts the existing {FILE}.enc and lists the keys being added (+), removed (-) and
changed (~), then asks for confirmation. Values are never printed. Without a terminal it refuses unless --yes.
`
	return strings.TrimSpace(helpText)
}

// Run the main execution of command
func (c *EncryptCommand) Run(args []string) int {
	// Init flag
	f := flag.NewFlagSet("enc", flag.ContinueOnError)
	f.StringVar(&c.File, "file", "", "file")
	f.StringVar(&c.Password, "password", "", "password")
	f.StringVar(&c.PSName, "ps-name", "", "ps-name")
	f.StringVar(&c.PSRegion, "ps-region", "us-east-1", "ps-region")
	f.BoolVar(&c.Yes, "yes", false, "yes")
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
		c.Password = getSecretsPassword()
	}

	// Validate Options
	fmt.Print("Validate Options...")
	if err := c.ValidateOptions(); err != nil {
		fmt.Println(err)
		return 1
	}
	fmt.Println("done")

	src := c.File
	dst := c.File + ".enc"
	password := []byte(c.Password)
	plaintext, err := os.ReadFile(src)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	reviewed, err := readIfExists(dst)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	write, err := reviewChanges(plaintext, reviewed, dst, password, c.Yes)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if !write {
		return 0
	}

	// Encrypt File
	fmt.Printf("Encrypt File...")
	ciphertext, err := encrypt(plaintext, password)
	if err == nil {
		err = writeUnchanged(dst, reviewed, ciphertext)
	}
	if err != nil {
		fmt.Println(err)
		return 1
	}
	fmt.Println("done")

	return 0
}

// readIfExists returns nil, not an error, when path does not exist.
func readIfExists(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	return data, err
}

// writeUnchanged writes ciphertext to dst only if dst still holds what was reviewed, so a change landing while the
// confirmation prompt waits (a git pull, another enc) is not overwritten unseen.
func writeUnchanged(dst string, reviewed, ciphertext []byte) error {
	now, err := readIfExists(dst)
	if err != nil {
		return err
	}
	if !bytes.Equal(now, reviewed) || (now == nil) != (reviewed == nil) {
		return fmt.Errorf("%s changed while waiting for confirmation, nothing written: run enc again", dst)
	}

	return writeFileAtomic(dst, ciphertext, 0o644)
}

// reviewChanges shows which keys plaintext adds, removes or changes against existing, the current contents of dst
// (nil when dst does not exist) — names only, never values — so whoever encrypts can see an edit they did not make,
// such as a key someone else added since this plaintext was decrypted. It reports whether to write dst; declining or
// lacking a terminal without yes is an error.
func reviewChanges(plaintext, existing []byte, dst string, password []byte, yes bool) (bool, error) {
	var current []byte
	if existing != nil {
		var err error
		if current, err = decrypt(existing, password); err != nil {
			return false, fmt.Errorf("decrypt current %s: %w", dst, err)
		}
	}

	// Encryption is randomized, so rewriting identical plaintext would still change every byte of dst.
	if existing != nil && bytes.Equal(plaintext, current) {
		fmt.Printf("%s already holds this plaintext, nothing to write\n", dst)
		return false, nil
	}

	changes := diffEnv(current, plaintext)
	fmt.Printf("Changes to %s:\n", dst)
	for _, key := range changes.Added {
		fmt.Printf("  + %s\n", key)
	}
	for _, key := range changes.Removed {
		fmt.Printf("  - %s\n", key)
	}
	for _, key := range changes.Changed {
		fmt.Printf("  ~ %s\n", key)
	}
	if changes.OtherLines > 0 {
		fmt.Printf("  ? %d line(s) not in KEY=VALUE form added or removed (contents not shown)\n", changes.OtherLines)
	}
	if changes.empty() {
		fmt.Println("  (no key changes, only comments, blank lines or order differ)")
	}

	if yes {
		return true, nil
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return false, errors.New("refusing to write without confirmation: rerun in a terminal or pass --yes")
	}
	fmt.Printf("Write %s? [y/N] ", dst)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if !strings.EqualFold(strings.TrimSpace(answer), "y") {
		return false, errors.New("aborted, nothing written")
	}

	return true, nil
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
