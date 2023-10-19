FROM golang:1.21-alpine as builder
WORKDIR /code
COPY go.* .
RUN go mod download
COPY main.go .
RUN GOOS=linux go build -ldflags '-s -w' -o leader-election main.go

FROM alpine:3.18
CMD ["./leader-election"]
COPY --from=builder /code/leader-election .
