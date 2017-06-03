all: protobuf

protobuf:
	protoc --mesh_out=plugins=grpc-mesh:$(GOPATH)/src --proto_path=$(GOPATH)/src $(CURDIR)/*.proto
