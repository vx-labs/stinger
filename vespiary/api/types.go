package api

//go:generate protoc -I ${GOPATH}/src/github.com/vx-labs/vespiary/vendor -I ${GOPATH}/src/github.com/vx-labs/vespiary/vendor/github.com/gogo/protobuf/ -I ${GOPATH}/src/github.com/vx-labs/vespiary/vespiary/api api.proto --go_out=plugins=grpc:.
