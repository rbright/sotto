set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

bin_dir := justfile_directory() + "/bin"
tooling_flake := "path:" + justfile_directory()

default:
  @just --list

tools:
  mkdir -p "{{bin_dir}}"
  GOBIN="{{bin_dir}}" go install github.com/bufbuild/buf/cmd/buf@v1.57.2
  GOBIN="{{bin_dir}}" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
  GOBIN="{{bin_dir}}" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
  GOBIN="{{bin_dir}}" go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

fmt:
  gofmt -w $(find apps/sotto -type f -name '*.go' -not -path '*/vendor/*')

fmt-check:
  test -z "$(gofmt -l $(find apps/sotto -type f -name '*.go' -not -path '*/vendor/*'))"

lint: tools
  "{{bin_dir}}/golangci-lint" run ./apps/sotto/...

test:
  go test ./apps/sotto/...

test-integration:
  go test -tags=integration ./apps/sotto/internal/audio -run Integration

generate:
  "{{bin_dir}}/buf" generate apps/sotto/proto/third_party --template buf.gen.yaml

ci:
  just fmt
  just lint
  just test

ci-check:
  just fmt-check
  just lint
  just test
  just generate
  git diff --exit-code -- apps/sotto/proto/gen/go

nix-build-check:
  nix build 'path:.#sotto'

nix-run-help-check:
  nix run 'path:.#sotto' -- --help

fmt-nix:
  nix develop '{{ tooling_flake }}' -c nixfmt flake.nix

lint-nix:
  nix develop '{{ tooling_flake }}' -c deadnix flake.nix
  nix develop '{{ tooling_flake }}' -c statix check flake.nix
  nix develop '{{ tooling_flake }}' -c nixfmt --check flake.nix

precommit-install:
  nix develop '{{ tooling_flake }}' -c prek install --hook-type pre-commit --hook-type pre-push

precommit-run:
  nix develop '{{ tooling_flake }}' -c prek run --all-files --hook-stage pre-commit

prepush-run:
  nix develop '{{ tooling_flake }}' -c prek run --all-files --hook-stage pre-push

smoke-riva-doctor:
  sotto doctor

smoke-riva-manual:
  @echo "Run this in an active Hyprland session with local Riva up:"
  @echo "  1) sotto doctor"
  @echo "  2) sotto toggle   # start recording"
  @echo "  3) speak a short phrase"
  @echo "  4) sotto toggle   # stop+commit"
  @echo "  5) verify clipboard/paste + cues"
  @echo "  6) sotto cancel   # verify cancel path"
