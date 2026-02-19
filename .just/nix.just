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
