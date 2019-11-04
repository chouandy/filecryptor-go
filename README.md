# File Cryptor

File Cryptor is a tool for encrypting, and decrypting file.

## Install

``` sh
go get -u github.com/chouandy/filecryptor
```

## Usage

The password can be ENV["SECRETS_PASSWORD"] or ENV["SECRETS_PASSWORD_PS_NAME"] + ENV["SECRETS_PASSWORD_PS_REGION"]

- Encrypt File

``` sh
filecryptor enc --file {FILE} --password {PASSWORD}
# or
filecryptor enc --file {FILE} --ps-name {PS_NAME} --ps-region {PS_REGION}
```

- Decrypt File

``` sh
filecryptor dec --file {FILE} --password {password}
# or
filecryptor dec --file {FILE} --ps-name {PS_NAME} --ps-region {PS_REGION}
```

- Random Hex String

``` sh
filecryptor hex -n {N}
```
