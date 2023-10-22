IMAGE_TAG=leader-election:$(shell date +%Y%m%d%H%M%S)

.PHONY: docker build
build:
	GOOS=linux go build -ldflags '-s -w' -o leader-election main.go

docker:
	docker build . -t $(IMAGE_TAG)