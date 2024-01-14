lint: src
	docker run --rm -v $$PWD/src:/app -w /app golangci/golangci-lint golangci-lint run -v
