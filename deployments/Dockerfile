ARG GO_VERSION=1

FROM golang:${GO_VERSION}-bookworm as builder
RUN apt-get update && apt-get install -y git
WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
ARG GIT_TAG=dev
RUN echo "GIT_TAG is ${GIT_TAG}" && go build -ldflags "-X 'main.VERSION=${GIT_TAG}'" -v -o /run-app ./cmd/recast

FROM debian:bookworm
RUN apt-get update && apt-get install -y ca-certificates tzdata && apt-get clean
COPY --from=builder /run-app /usr/local/bin/
COPY --from=builder /usr/src/app/migrations /migrations
EXPOSE 6060 8080
CMD ["run-app"]
