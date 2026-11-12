# filecryptor

A small CLI for keeping dotenv files encrypted in git. Commit `.env.prod.enc`, keep `.env.prod` out of the repo, and
decrypt it at deploy or container start with a password read from AWS SSM Parameter Store.

- **AES-256-GCM** with a PBKDF2-derived key and a random nonce per encryption
- **Password from SSM Parameter Store**, so nothing secret lives in the repo or the image
- **Change review on encrypt**: shows which keys you are adding, removing or changing before overwriting the
  committed file, so an edit made from a stale copy cannot silently erase someone else's change

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Usage](#usage)
  - [Password](#password)
  - [enc](#enc)
  - [dec](#dec)
  - [hex](#hex)
- [Using It in a Container](#using-it-in-a-container)
- [Changing the Password](#changing-the-password)
- [File Format](#file-format)
- [Development](#development)

## Installation

Requires Go 1.21 or newer.

```sh
go install github.com/chouandy/filecryptor@latest
```

Pin a version in Dockerfiles and CI rather than using `@latest`.

## Quick Start

```sh
# 1. Decrypt the committed file to edit it
filecryptor dec --file .env.prod.enc --ps-name /my-app/secrets-password --ps-region eu-central-1

# 2. Edit .env.prod

# 3. Encrypt it again: review the key changes, then confirm
filecryptor enc --file .env.prod --ps-name /my-app/secrets-password --ps-region eu-central-1
```

Always decrypt the latest committed `.enc` right before editing. Reusing an old plaintext copy is how changes get lost.

## Usage

```
filecryptor <command> [options]
```

| Command | Description |
|---------|-------------|
| `enc`   | Encrypt `{FILE}` to `{FILE}.enc` |
| `dec`   | Decrypt `{FILE}.enc` to `{FILE}` |
| `hex`   | Print a random hex string |

### Password

`enc` and `dec` take the password from the first of these that is set:

1. `--password`
2. `SECRETS_PASSWORD`
3. The SSM SecureString parameter named by `--ps-name` / `SECRETS_PASSWORD_PS_NAME`, in the region given by
   `--ps-region` / `SECRETS_PASSWORD_PS_REGION`

Reading from SSM uses the standard AWS credential chain (`AWS_PROFILE`, environment variables, the ECS task role and
so on) and needs `ssm:GetParameter` on that parameter, plus `kms:Decrypt` when it is encrypted with a customer-managed
KMS key.

`--ps-region` defaults to `us-east-1` when `--ps-name` is given without it.

### enc

```sh
filecryptor enc --file .env.prod --ps-name /my-app/secrets-password --ps-region eu-central-1
```

| Option        | Description |
|---------------|-------------|
| `--file`      | Plaintext file to encrypt. Output goes to `{FILE}.enc`. |
| `--password`  | Encryption password. |
| `--ps-name`   | SSM parameter that holds the password. |
| `--ps-region` | Region of that parameter. |
| `--yes`       | Write without asking. The key changes are still printed. |

Before writing, `enc` decrypts the existing `{FILE}.enc` in memory and lists what the new plaintext changes, by key
name only. Values are never printed.

```
Changes to .env.prod.enc:
  + COINGECKO_API_KEY
  ~ ZEROX_API_KEYS
Write .env.prod.enc? [y/N]
```

| Mark | Meaning |
|------|---------|
| `+`  | Key added |
| `-`  | Key removed |
| `~`  | Key kept, value changed |
| `?`  | Other lines added or removed, counted only, contents never shown |

Only upper-case names (`[A-Z_][A-Z0-9_]*`) are listed by name. Any other line, including `lower_case=...` or an
unquoted base64 line ending in `=`, could hold secret material, so it is only counted.

A quoted value that spans several lines (a PEM key, for example) belongs to its key, so editing it shows as `~ KEY`.

If a key you did not touch shows up as `-` or `~`, your plaintext was decrypted from an older `.enc`. Answer `N`,
decrypt again, and redo your edit.

It also:

- Writes nothing when the plaintext matches the existing `.enc` exactly. Encryption is randomized, so rewriting would
  otherwise change the whole file for no reason.
- Refuses to overwrite a `.enc` it cannot decrypt, for example with the wrong password.
- Aborts if `{FILE}.enc` changes while the prompt is waiting, for example from a `git pull`.
- Replaces `{FILE}.enc` atomically, so a failed write leaves the old file intact. The exception is a file that cannot
  be replaced by rename (a directory that does not allow creating files, or a bind mount): it is then overwritten in
  place, as `dec` does below, and an interrupted write can leave it truncated.
- Refuses to write without a terminal unless `--yes` is passed, so scripts and agents must opt in explicitly.

### dec

```sh
filecryptor dec --file .env.prod.enc --ps-name /my-app/secrets-password --ps-region eu-central-1
```

`--file` must end in `.enc`. The plaintext is written to the same path without the suffix, with mode `0600`, owned by
the user running `dec`. An existing plaintext file is replaced atomically; if it is a symlink, the file it points to
is replaced and the link is kept. A symlink to a missing file is an error. If the file cannot be replaced that way (a root-owned
app directory holding a plaintext file pre-created for a non-root user, or a bind-mounted file), `dec` overwrites it
in place instead, as earlier versions did, after narrowing its mode to `0600`. If it cannot narrow the mode of a file
others can read, it writes nothing and fails.

`dec` takes the same `--file`, `--password`, `--ps-name` and `--ps-region` options as `enc`.

### hex

```sh
filecryptor hex -n 32   # 32 random bytes, printed as 64 hex characters
```

The bytes come from the OS's cryptographically secure source, so the output is fit to use as a password.

| Option | Default | Description |
|--------|---------|-------------|
| `-n`   | `16`    | Number of random bytes |

## Using It in a Container

Bake the `.enc` file into the image and decrypt it at startup with the task role:

```dockerfile
FROM golang:1.21-alpine AS filecryptor
RUN go install github.com/chouandy/filecryptor@vX.Y.Z

FROM alpine
COPY --from=filecryptor /go/bin/filecryptor /app/filecryptor
COPY .env.prod.enc /app/
```

```sh
# entrypoint.sh
./filecryptor dec --file ".env.${STAGE}.enc"
set -a; . ".env.${STAGE}"; set +a
exec "$@"
```

Set `SECRETS_PASSWORD_PS_NAME` and `SECRETS_PASSWORD_PS_REGION` on the container.

The plaintext is `0600`, so run `dec` as the same user as the app. If the entrypoint runs as root and drops privileges
before starting the app, `chown` the file to that user after `dec`.

## Changing the Password

`enc` refuses to overwrite a `.enc` it cannot decrypt, so rotate the password in three steps:

```sh
filecryptor dec --file .env.prod.enc --password "$OLD"   # 1. decrypt with the old password
rm .env.prod.enc                                         # 2. remove the old encrypted file
filecryptor enc --file .env.prod --password "$NEW"       # 3. encrypt with the new one
```

Then update the password in SSM and redeploy everything that decrypts this file.

## File Format

```
[ AES-256-GCM ciphertext + tag ][ 12-byte nonce ]
```

The key is `PBKDF2-HMAC-SHA1(password, nonce, 4096 iterations, 32 bytes)`. The nonce doubles as the salt and is
random for every encryption. Files written by earlier versions decrypt unchanged.

## Development

```sh
go test ./...
make dev     # build bin/filecryptor
make build   # build bin/filecryptor-linux and bin/filecryptor-darwin
```
