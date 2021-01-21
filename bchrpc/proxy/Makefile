all: gw web/bchrpc.swagger.json

gw: ../pb/bchrpc.pb.go ../pb/bchrpc.pb.gw.go gw.go
	go build -o gw -v

../pb/bchrpc.pb.go: ../bchrpc.proto
	protoc -I../ \
		--go_out=plugins=grpc:../pb \
		--go_opt=paths=source_relative \
		../bchrpc.proto

../pb/bchrpc.pb.gw.go: ../bchrpc.proto ./bchrpc.yaml
	protoc \
		-I../ \
		--grpc-gateway_out=:../pb \
		--grpc-gateway_opt grpc_api_configuration=./bchrpc.yaml \
		--grpc-gateway_opt logtostderr=true \
		--grpc-gateway_opt paths=source_relative \
		--grpc-gateway_opt generate_unbound_methods=true \
		../bchrpc.proto

web/bchrpc.swagger.json: ../bchrpc.proto ./bchrpc.yaml
	protoc -I ../ \
		--openapiv2_out ./web \
		--openapiv2_opt logtostderr=true \
		--openapiv2_opt grpc_api_configuration=./bchrpc.yaml \
		--openapiv2_opt generate_unbound_methods=true \
		--openapiv2_opt json_names_for_fields=false \
		../bchrpc.proto

