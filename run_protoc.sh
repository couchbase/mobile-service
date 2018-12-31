protoc -I mobile_service_grpc/ mobile_service_grpc/common.proto --go_out=plugins=grpc:mobile_service_grpc
protoc -I mobile_service_grpc/ mobile_service_grpc/mobile_service.proto --go_out=plugins=grpc:mobile_service_grpc
protoc -I mobile_service_grpc/ mobile_service_grpc/mobile_service_admin.proto --go_out=plugins=grpc:mobile_service_grpc
