{
  description = "sotto: local-first ASR CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        version = if self ? shortRev then "0.1.0-${self.shortRev}" else "0.1.0-dev";
      in
      {
        packages = rec {
          sotto = pkgs.buildGoModule {
            pname = "sotto";
            inherit version;
            src = ./.;
            modRoot = "apps/sotto";
            go = pkgs.go_1_25;
            subPackages = [ "cmd/sotto" ];
            vendorHash = "sha256-4/+DtLMcMwhckIH+ieVlsleXxzdA+J1kYXrpzVmW52s=";
            env.GOWORK = "off";
            ldflags = [
              "-s"
              "-w"
              "-X github.com/rbright/sotto/internal/version.Version=${version}"
              "-X github.com/rbright/sotto/internal/version.Commit=${self.shortRev or "dirty"}"
              "-X github.com/rbright/sotto/internal/version.Date=unknown"
            ];
            nativeBuildInputs = [ pkgs.makeWrapper ];
            postInstall = ''
              wrapProgram $out/bin/sotto \
                --prefix PATH : ${
                  pkgs.lib.makeBinPath [
                    pkgs.curl
                    pkgs.hyprland
                    pkgs.pipewire
                    pkgs.wl-clipboard
                  ]
                }
            '';
          };

          default = sotto;
        };

        apps = {
          sotto = flake-utils.lib.mkApp { drv = self.packages.${system}.sotto; };
          default = self.apps.${system}.sotto;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_25
            just
            buf
            protobuf
            golangci-lint
            statix
            deadnix
            nixfmt-rfc-style
            prek
          ];
        };

        formatter = pkgs.nixfmt-rfc-style;
      }
    )
    // {
      nixosModules.default =
        {
          config,
          lib,
          pkgs,
          ...
        }:
        {
          options.programs.sotto.enable = lib.mkEnableOption "Install the sotto CLI";

          config = lib.mkIf config.programs.sotto.enable {
            environment.systemPackages = [ self.packages.${pkgs.system}.sotto ];
          };
        };
    };
}
