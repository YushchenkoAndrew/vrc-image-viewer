.PHONY: dev build migrate test cover swagger

dev: 
	go run main.go

build: 
	go build -o ./vr-sync main.go

test:
	config="$(PWD)" go test -count=1 -v ./...

swagger:
	swag init -g ./main.go --parseDependency

cover:
	go test ./... -coverprofile=testprofile.out
	go tool cover -html=testprofile.out
