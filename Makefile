.PHONY: all clean build test vendor

all: clean build test

vendor:
	go mod tidy -v
	rm -fr vendor
	go mod vendor

build:
	mkdir -p bin
	go build -mod=vendor -o bin/test_main      test/main.go
	go build -mod=vendor -o bin/raft_kv_store  cmd/main.go

clean:
	rm -rf bin

test:
	./bin/test_main
