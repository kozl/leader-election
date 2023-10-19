IMAGE_TAG=leader-election:latest

.PHONY: docker build
build:
	GOOS=linux go build -ldflags '-s -w' -o leader-election main.go

docker:
	docker build . -t $(IMAGE_TAG)