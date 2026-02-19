bin_dir := justfile_directory() + "/bin"
tooling_flake := "path:" + justfile_directory()

default:
  @just --list
