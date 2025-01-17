.PHONY : api gserver gclient build

build: api gserver gclient

api: 
	protoc -I api/ --go_out=plugins=grpc:api api/*.proto

new-key:
	openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.cert -days 3650 -nodes -subj '/CN=daboss'

gserver:
	go build -o gserver server/*.go

gclient:
	go build -o gclient client/*.go
