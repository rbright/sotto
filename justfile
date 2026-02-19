set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

import ".just/common.just"
import ".just/go.just"
import ".just/codegen.just"
import ".just/ci.just"
import ".just/nix.just"
import ".just/hooks.just"
import ".just/smoke.just"
