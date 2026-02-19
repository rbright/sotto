precommit-install:
  nix develop '{{ tooling_flake }}' -c prek install --hook-type pre-commit --hook-type pre-push

precommit-run:
  nix develop '{{ tooling_flake }}' -c prek run --all-files --hook-stage pre-commit

prepush-run:
  nix develop '{{ tooling_flake }}' -c prek run --all-files --hook-stage pre-push
