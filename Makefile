all: spec test image

spec: main.go
	go build -o spec main.go

test:
	go test

run: spec
	./spec

image: Dockerfile spec
	docker build .