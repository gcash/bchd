all: gw web/bchrpc.swagger.json

gw: gen/bchrpc.pb.go gen/bchrpc.pb.gw.go
	go build -o gw -v

gen/bchrpc.pb.go: ../bchrpc.proto
	protoc \
		-I/usr/local/include \
		-I../ \
		-I${GOPATH}/src \
		-I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
		--go_out=plugins=grpc:gen \
		../bchrpc.proto

gen/bchrpc.pb.gw.go: ../bchrpc.proto ./bchrpc.yaml
	protoc \
		-I/usr/local/include \
		-I../ \
		-I${GOPATH}/src \
		-I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
		--grpc-gateway_out=logtostderr=true,grpc_api_configuration=./bchrpc.yaml:gen \
		../bchrpc.proto

web/bchrpc.swagger.json: ../bchrpc.proto ./bchrpc.yaml
	protoc \
		-I/usr/local/include \
		-I../ \
		-I${GOPATH}/src \
		-I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
		--swagger_out=logtostderr=true,grpc_api_configuration=./bchrpc.yaml:web \
		../bchrpc.proto
