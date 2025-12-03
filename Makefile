.PHONY: all
all: build

.PHONY: build
build:
	go build -gcflags='all=-N -l' -o rulemanager ./cmd/rulemanager

.PHONY: install
install:
	go install

.PHONY: run
run: build
	./rulemanager

.PHONY: unit-test
unit-test:
	go test ./... -short

.PHONY: test
test: unit-test

.PHONY: generate
generate:
	go generate ./...

.PHONY: format
format:
	gofumpt -l -w .

.PHONY: lint
lint:
	golangci-lint run

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix

.PHONY: vendor
vendor:
	go mod vendor && go mod tidy
