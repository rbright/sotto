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
