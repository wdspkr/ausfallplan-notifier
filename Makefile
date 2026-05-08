.PHONY: build deploy invoke clean test

BUILD_DIR := build

build:
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags='-s -w' -o $(BUILD_DIR)/bootstrap ./cmd/lambda

deploy: build
	sam deploy

invoke:
	aws lambda invoke --function-name ausfallplan-check /dev/stdout

clean:
	rm -rf $(BUILD_DIR) .aws-sam

test:
	go test ./...
