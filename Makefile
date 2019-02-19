dev:
	go build -ldflags="-s -w" -o bin/filecryptor *.go

build:
	env GOOS=linux go build -ldflags="-s -w" -o bin/filecryptor-linux *.go
	env GOOS=darwin go build -ldflags="-s -w" -o bin/filecryptor-darwin *.go
