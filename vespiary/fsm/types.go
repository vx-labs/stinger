package fsm

//go:generate protoc -I${GOPATH}/src -I ${GOPATH}/src/github.com/vx-labs/vespiary/vendor -I ${GOPATH}/src/github.com/vx-labs/vespiary/vendor/github.com/gogo/protobuf/  -I${GOPATH}/src/github.com/vx-labs/vespiary/vespiary/fsm/ --go_out=plugins=grpc:. types.proto
