# Promenljive
main_package_path = ./cmd/stream
binary_name = sstream

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

.PHONY: help
help:
	@echo "Usage:"
	@echo "  make build     - Build the application"
	@echo "  make run       - Run the application"
	@echo "  make test      - Run tests"
	@echo "  make tidy      - Tidy modfiles and format code"

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## build: Build the application
.PHONY: build
build:
	go build -o=/tmp/bin/${binary_name} ${main_package_path}

## run: Run the application
.PHONY: run
run: build
	/tmp/bin/${binary_name}

## test: Run all tests
.PHONY: test
test:
	go test -v -race ./...

## tidy: Tidy modfiles and format code
.PHONY: tidy
tidy:
	go mod tidy
	go fmt ./...
