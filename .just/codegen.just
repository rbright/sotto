generate: tools
  PATH="{{bin_dir}}:$PATH" "{{bin_dir}}/buf" generate apps/sotto/proto/third_party --template buf.gen.yaml
