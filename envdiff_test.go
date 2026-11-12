package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiffEnv(t *testing.T) {
	before := []byte("# keys\nKEEP=1\nCHANGE=old\nDROP=x\nexport EXPORTED=a\nDUP=first\nDUP=second\n")
	after := []byte("KEEP=1\nCHANGE=new\n\nNEW=y\nEXPORTED=a\nDUP=second\n")

	got := diffEnv(before, after)

	want := keyChanges{Added: []string{"NEW"}, Removed: []string{"DROP"}, Changed: []string{"CHANGE"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestDiffEnvFormattingOnly(t *testing.T) {
	got := diffEnv([]byte("A=1\nB=2\n"), []byte("# comment\nB=2\n\nA=1\n"))

	if !got.empty() {
		t.Fatalf("diffEnv = %+v, want no key changes", got)
	}
}

func encrypted(t *testing.T, plaintext []byte) []byte {
	t.Helper()
	data, err := encrypt(plaintext, []byte("pw"))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func TestReviewChangesSkipsIdenticalPlaintext(t *testing.T) {
	write, err := reviewChanges([]byte("A=1\n"), encrypted(t, []byte("A=1\n")), ".env.enc", []byte("pw"), true)
	if err != nil {
		t.Fatal(err)
	}
	if write {
		t.Fatal("reviewChanges asked to rewrite identical plaintext")
	}
}

func TestReviewChangesWritesWithYes(t *testing.T) {
	write, err := reviewChanges([]byte("A=1\nB=2\n"), encrypted(t, []byte("A=1\n")), ".env.enc", []byte("pw"), true)
	if err != nil {
		t.Fatal(err)
	}
	if !write {
		t.Fatal("reviewChanges declined a changed plaintext under --yes")
	}
}

func TestReviewChangesWritesNewFileWithYes(t *testing.T) {
	write, err := reviewChanges([]byte("A=1\n"), nil, ".env.enc", []byte("pw"), true)
	if err != nil {
		t.Fatal(err)
	}
	if !write {
		t.Fatal("reviewChanges declined to create a new encrypted file under --yes")
	}
}

func TestReviewChangesRejectsUndecryptableTarget(t *testing.T) {
	_, err := reviewChanges([]byte("A=2\n"), encrypted(t, []byte("A=1\n")), ".env.enc", []byte("other"), true)
	if err == nil {
		t.Fatal("reviewChanges overwrote a file it could not decrypt")
	}
}

func TestDiffEnvMultiLineValueBelongsToItsKey(t *testing.T) {
	before := []byte("PEM=\"-----BEGIN-----\nMIIEsecretQ==\n-----END-----\"\nAFTER=1\n")
	after := []byte("PEM=\"-----BEGIN-----\nMIIEotherQ==\n-----END-----\"\nAFTER=1\n")

	got := diffEnv(before, after)

	want := keyChanges{Changed: []string{"PEM"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestDiffEnvQuoteHandling(t *testing.T) {
	// A="x\"y" closes on its own line despite the escaped quote; B='a\' closes because single quotes take no escapes.
	before := []byte("A=\"x\\\"y\"\nB='a\\'\nC=1\n")
	after := []byte("A=\"x\\\"y\"\nB='a\\'\nC=2\n")

	got := diffEnv(before, after)

	want := keyChanges{Changed: []string{"C"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestDiffEnvQuotedBlockNeverYieldsKeys(t *testing.T) {
	// "export  PEM" (two spaces) and "BAD KEY" still open a quote, so their continuation lines stay in the block.
	before := []byte("export  PEM=\"-----BEGIN-----\nMIIEsecretQ==\n-----END-----\"\nBAD KEY=\"x\nSECRETLINE=y\"\n")
	after := []byte("export  PEM=\"-----BEGIN-----\nMIIEotherQ==\n-----END-----\"\nBAD KEY=\"x\nSECRETLINE=z\"\n")

	got := diffEnv(before, after)

	// PEM is a valid key once the export prefix is trimmed. The BAD KEY block is one non-key block that changed:
	// one block gone and one new.
	want := keyChanges{Changed: []string{"PEM"}, OtherLines: 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestDiffEnvKeepsTrailingSpaceInMultiLineValue(t *testing.T) {
	got := diffEnv([]byte("A=\"first  \nsecond\"\n"), []byte("A=\"first\nsecond\"\n"))

	want := keyChanges{Changed: []string{"A"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestDiffEnvApostropheInCommentOpensNoQuote(t *testing.T) {
	got := diffEnv([]byte("A=1 # don't\nB=1\n"), []byte("A=1 # don't\n"))

	want := keyChanges{Removed: []string{"B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestDiffEnvNeverNamesNonKeyLines(t *testing.T) {
	before := []byte("A: 1\nB: 2\n")
	after := []byte("A: 1\n\"password\": \"c2VjcmV0=\"\n")

	got := diffEnv(before, after)

	// Gone: "B: 2". New: the "password" line. Neither has a shell-name key, so both are counted, never named.
	want := keyChanges{OtherLines: 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestWriteUnchangedRefusesChangedTarget(t *testing.T) {
	dst := filepath.Join(t.TempDir(), ".env.enc")
	if err := os.WriteFile(dst, []byte("newer"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeUnchanged(dst, []byte("reviewed"), []byte("cipher")); err == nil {
		t.Fatal("writeUnchanged overwrote a file that changed after review")
	}
	if err := writeUnchanged(dst, nil, []byte("cipher")); err == nil {
		t.Fatal("writeUnchanged overwrote a file created after review")
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "newer" {
		t.Fatalf("dst = %q, want it untouched", got)
	}
}

func TestWriteUnchangedWritesReviewedTarget(t *testing.T) {
	dst := filepath.Join(t.TempDir(), ".env.enc")
	if err := os.WriteFile(dst, []byte("reviewed"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeUnchanged(dst, []byte("reviewed"), []byte("cipher")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "cipher" {
		t.Fatalf("dst = %q, want %q", got, "cipher")
	}
}

func TestDiffEnvNeverNamesBase64Lines(t *testing.T) {
	got := diffEnv([]byte("TOKEN=abc\n"), []byte("TOKEN=abc\nc2VjcmV0VmFsdWU=\nlower_key=1\n"))

	// Both new lines are valid shell assignments, but neither name is upper case, so both are counted, not named.
	want := keyChanges{OtherLines: 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestDiffEnvQuoteInsideUnquotedValueOpensNothing(t *testing.T) {
	before := []byte("PW=it's\nA=1\nNEW=2\nZ='q'\n")
	after := []byte("PW=it's-changed\nA=1\nZ='q'\n")

	got := diffEnv(before, after)

	want := keyChanges{Removed: []string{"NEW"}, Changed: []string{"PW"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestDiffEnvStrayQuoteAfterClosedValueOpensNothing(t *testing.T) {
	got := diffEnv([]byte("A=\"x\" it's\nB=1\nC=2\n"), []byte("A=\"x\" it's\nC=2\n"))

	want := keyChanges{Removed: []string{"B"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}

func TestDiffEnvNamesKeyWithSpaceBeforeEquals(t *testing.T) {
	got := diffEnv([]byte("A = 1\n"), []byte("A = 9\n"))

	want := keyChanges{Changed: []string{"A"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffEnv = %+v, want %+v", got, want)
	}
}
