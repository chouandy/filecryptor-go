package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/crypto/pbkdf2"
)

// The encrypted file layout is AES-256-GCM ciphertext followed by its 12-byte
// nonce. The nonce doubles as the PBKDF2 salt, so every encryption of the same
// plaintext yields different bytes.
const nonceSize = 12

func deriveKey(password, nonce []byte) []byte {
	return pbkdf2.Key(password, nonce, 4096, 32, sha1.New)
}

func newGCM(password, nonce []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(deriveKey(password, nonce))
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

func encrypt(plaintext, password []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	aead, err := newGCM(password, nonce)
	if err != nil {
		return nil, err
	}

	return append(aead.Seal(nil, nonce, plaintext, nil), nonce...), nil
}

func decrypt(data, password []byte) ([]byte, error) {
	if len(data) < nonceSize {
		return nil, errors.New("encrypted file is too short")
	}
	body, nonce := data[:len(data)-nonceSize], data[len(data)-nonceSize:]
	aead, err := newGCM(password, nonce)
	if err != nil {
		return nil, err
	}

	return aead.Open(nil, nonce, body, nil)
}

// decryptFile writes the plaintext owner-only: it holds the secrets.
func decryptFile(src, dst string, password []byte) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	plaintext, err := decrypt(data, password)
	if err != nil {
		return err
	}

	return writeFileAtomic(dst, plaintext, 0o600)
}

// writeFileAtomic replaces path, or the file it links to, with data in one rename, so a failed write leaves the old
// file intact. The result is a new file owned by the caller with mode perm, whatever the old file's owner and mode
// were. When the file cannot be replaced by rename, it falls back to writeInPlace.
func writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	// Write through a symlink, as a plain open would, rather than replacing the link itself.
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	} else if info, lerr := os.Lstat(path); lerr == nil && info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink to a missing file, nothing written", path)
	}

	// The temp name keeps the target's prefix, so a .env* ignore rule also covers a copy left by a killed process.
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if errors.Is(err, fs.ErrPermission) {
		return writeInPlace(path, data, perm)
	}
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed

	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(perm)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	// A bind-mounted file (EBUSY) or another user's file in a sticky directory (EPERM) cannot be renamed over.
	if err := os.Rename(tmp.Name(), path); errors.Is(err, fs.ErrPermission) || errors.Is(err, syscall.EBUSY) {
		return writeInPlace(path, data, perm)
	} else if err != nil {
		return err
	}

	return nil
}

// writeInPlace is the fallback for an existing file that cannot be replaced by rename, such as one pre-created for a
// non-root user in a root-owned app directory, or a bind-mounted file: it overwrites the file directly, as plain open
// does, so it is not atomic. Checking, narrowing the mode to perm and writing all go through one open handle, so the
// file cannot be swapped in between. When the caller cannot change the mode and the file allows more than perm,
// nothing is written.
func writeInPlace(path string, data []byte, perm fs.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file, nothing written", path)
	}
	if err := f.Chmod(perm); err != nil && info.Mode().Perm()&^perm != 0 {
		return fmt.Errorf("cannot restrict %s to mode %v before writing, nothing written: %w", path, perm, err)
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}

	return f.Close()
}
