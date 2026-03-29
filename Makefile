.PHONY: proto build clean test

PROTO_FILES := $(shell find proto -name '*.proto')
GO_BINS := cmd/initd cmd/climain

proto:
	PATH=$$PATH:$$HOME/go/bin protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

build: proto
	@for dir in $(GO_BINS); do \
		echo "Building $$dir..."; \
		go build -trimpath -ldflags="-s -w" -o bin/$$(basename $$dir) ./$$dir; \
	done

clean:
	rm -rf bin/
	find proto -name '*.pb.go' -delete

test:
	go test ./...
