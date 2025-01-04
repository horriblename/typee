{
  description = "A very basic flake";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    go-sumtype.url = "github:BurntSushi/go-sumtype";
    go-sumtype.flake = false;
  };
  outputs = {
    self,
    nixpkgs,
    ...
  } @ inputs: let
    inherit (nixpkgs) lib;
    eachSystem = lib.genAttrs ["x86_64-linux"];
    pkgsFor = eachSystem (
      system:
        import nixpkgs {
          localSystem = system;
          overlays = [self.overlays.default];
        }
    );
    goFixedVersion = "1.23.2";
  in {
    overlays = {
      default = final: _prev: {
        hello = final.callPackage ./hello.nix {};
        go-sumtype = final.callPackage ./nix/go-sumtype.nix {
          source = inputs.go-sumtype;
          version = "master";
        };
        go123 = final.go_1_22.overrideAttrs (old: {
          version = goFixedVersion;
          src = final.runCommand "gowasi-version-hack" {} ''
            mkdir -p $out
            echo "go-${goFixedVersion}" > $out/VERSION
            cp -vrf ${inputs.go123}/* $out
          '';
        });
      };
    };

    packages = eachSystem (system: {
      default = self.packages.${system}.hello;
      inherit (pkgsFor.${system}) hello go-sumtype go123;
    });
    devShells = eachSystem (system: let
      pkgs = pkgsFor.${system};
    in {
      default = pkgs.mkShell {
        nativeBuildInputs = with pkgs; [
          go_1_23
          go-sumtype
          qbe

          pkg-config
          gobject-introspection
          glib
        ];
      };
    });
  };
}
