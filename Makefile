.PHONY: build test clean run-server run-scan lint

GO := GOTOOLCHAIN=local go
BIN_DIR := bin

build:
	$(GO) build -o $(BIN_DIR)/driftctl ./cmd/driftctl
	$(GO) build -o $(BIN_DIR)/driftd ./cmd/driftd

test:
	$(GO) test ./... -count=1

clean:
	rm -rf $(BIN_DIR)

run-server: build
	./$(BIN_DIR)/driftd -c configs/drift.example.yaml

run-scan: build
	./$(BIN_DIR)/driftctl scan --state testdata/sample.tfstate --provider aws --region us-east-1 --no-console || true

lint:
	$(GO) vet ./...

mod:
	$(GO) mod tidy
