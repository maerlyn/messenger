clean:
	rm messenger

run:
	go run *.go

fmt:
	go fmt *.go
	go fmt fb/*.go

build:
	go build -o messenger *.go
