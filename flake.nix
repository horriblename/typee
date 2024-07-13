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
  in {
    overlays = {
      default = final: _prev: {
        hello = final.callPackage ./hello.nix {};
        go-sumtype = final.callPackage ./nix/go-sumtype.nix {
          source = inputs.go-sumtype;
          version = "master";
        };
      };
    };

    packages = eachSystem (system: {
      default = self.packages.${system}.hello;
      inherit (pkgsFor.${system}) hello go-sumtype;
    });
    devShells = eachSystem (system: let
      pkgs = pkgsFor.${system};
    in {
      default = pkgs.mkShell {
        nativeBuildInputs = with pkgs; [
          go
          go-sumtype
        ];
      };
    });
  };
}
