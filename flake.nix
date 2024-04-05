{
  description = "A very basic flake";
  inputs = {
    opal = {
      url = "github:pyrocat101/opal";
      flake = false;
    };
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
      default = final: prev: {
        hello = final.callPackage ./hello.nix {};
        testy = final.callPackage ./test.nix {};
      };
    };

    packages = eachSystem (system: {
      default = self.packages.${system}.hello;
      inherit (pkgsFor.${system}) hello testy;
    });
    devShells = eachSystem (system: let
      pkgs = pkgsFor.${system};
    in {
      default = pkgs.mkShell {
        nativeBuildInputs = with pkgs; [
          zig
          glibc
          libgcc
          meson
          ninja
          gcc
          pkg-config
          libgccjit
        ];
        buildInputs = with pkgs; [
          libgccjit
          glibc
        ];
      };
    });
  };
}
