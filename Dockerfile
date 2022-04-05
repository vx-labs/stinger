FROM golang:alpine as builder
ENV CGO_ENABLED=0
RUN mkdir -p $GOPATH/src/github.com/vx-labs
WORKDIR $GOPATH/src/github.com/vx-labs/vespiary
COPY go.* ./
RUN go mod download
COPY . ./
RUN go test ./...
ARG BUILT_VERSION="snapshot"
RUN go build -buildmode=exe -ldflags="-s -w -X github.com/vx-labs/vespiary/cmd/vespiary/version.BuiltVersion=${BUILT_VERSION}" \
       -a -o /bin/vespiary ./cmd/vespiary

FROM alpine:3.15.4 as prod
ENTRYPOINT ["/usr/bin/vespiary"]
RUN apk -U add ca-certificates && \
    rm -rf /var/cache/apk/*
COPY --from=builder /bin/vespiary /usr/bin/vespiary
