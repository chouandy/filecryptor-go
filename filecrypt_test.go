package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// testdata/v1.5.0.env.enc was produced by `filecryptor enc` v1.5.0 with this password, so decrypting it proves
// existing .enc files still open after the crypto moved in-tree.
const fixturePassword = "fixture-password"

func TestDecryptV150File(t *testing.T) {
	data, err := os.ReadFile("testdata/v1.5.0.env.enc")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/v1.5.0.env")
	if err != nil {
		t.Fatal(err)
	}

	got, err := decrypt(data, []byte(fixturePassword))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("plaintext = %q, want %q", got, want)
	}
}

func TestEncryptRoundTrip(t *testing.T) {
	plaintext := []byte("A=1\nB=2\n")

	data, err := encrypt(plaintext, []byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := decrypt(data, []byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("plaintext = %q, want %q", got, plaintext)
	}
}

func TestDecryptRejectsWrongPassword(t *testing.T) {
	data, err := os.ReadFile("testdata/v1.5.0.env.enc")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := decrypt(data, []byte("wrong")); err == nil {
		t.Fatal("decrypt with the wrong password succeeded")
	}
}

func TestDecryptRejectsShortInput(t *testing.T) {
	if _, err := decrypt(make([]byte, nonceSize-1), []byte("pw")); err == nil {
		t.Fatal("decrypt of input shorter than the nonce succeeded")
	}
}

func TestDecryptFileTightensExistingPlaintextMode(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, ".env.enc"), filepath.Join(dir, ".env")
	data, err := encrypt([]byte("A=1\n"), []byte("pw"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := decryptFile(src, dst, []byte("pw")); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("dir has %d entries, want 2 (no temp file left behind)", len(entries))
	}
}

func TestDecryptRequiresEncSuffix(t *testing.T) {
	for _, file := range []string{"secrets", "dir/.enc", "a.enc.bak"} {
		c := &DecryptCommand{File: file, Password: "pw"}
		if err := c.ValidateOptions(); err == nil {
			t.Fatalf("ValidateOptions(%q) accepted a file that would decrypt onto itself or a bad name", file)
		}
	}
	c := &DecryptCommand{File: "dir/.env.prod.enc", Password: "pw"}
	if err := c.ValidateOptions(); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFileAtomicWritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target, link := filepath.Join(dir, "real"), filepath.Join(dir, "link")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(link, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Fatal("link was replaced by a regular file")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("target = %q, want %q", got, "new")
	}
}

func TestWriteFileAtomicRefusesDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(filepath.Join(dir, "missing"), link); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(link, []byte("new"), 0o600); err == nil {
		t.Fatal("writeFileAtomic replaced a dangling symlink")
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Fatal("link was replaced by a regular file")
	}
}

func TestWriteFileAtomicFallsBackInReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if err := writeFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("file = %q, want %q", got, "new")
	}
}

func TestWriteInPlaceRefusesNonRegularFile(t *testing.T) {
	if err := writeInPlace(os.DevNull, []byte("secret"), 0o600); err == nil {
		t.Fatal("writeInPlace wrote into a device file")
	}
}
