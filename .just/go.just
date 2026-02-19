# Install pinned developer tools into ./bin.
tools:
  mkdir -p "{{bin_dir}}"
  GOBIN="{{bin_dir}}" go install github.com/bufbuild/buf/cmd/buf@v1.57.2
  GOBIN="{{bin_dir}}" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
  GOBIN="{{bin_dir}}" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
  GOBIN="{{bin_dir}}" go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

# Format Go sources (excluding generated vendor paths).
fmt:
  bash -euo pipefail -c 'mapfile -t files < <(find apps/sotto -type f -name "*.go" -not -path "*/vendor/*"); if [[ "${#files[@]}" -eq 0 ]]; then exit 0; fi; gofmt -w "${files[@]}"'

# Check Go formatting only (no changes).
fmt-check:
  bash -euo pipefail -c 'mapfile -t files < <(find apps/sotto -type f -name "*.go" -not -path "*/vendor/*"); if [[ "${#files[@]}" -eq 0 ]]; then exit 0; fi; test -z "$(gofmt -l "${files[@]}")"'

lint: tools
  "{{bin_dir}}/golangci-lint" run ./apps/sotto/...

test:
  go test ./apps/sotto/...

test-integration:
  go test -tags=integration ./apps/sotto/internal/audio -run Integration
