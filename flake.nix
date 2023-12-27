{
  description = "A very basic flake";
  inputs = {
    roc.url = "github:roc-lang/roc";
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
      };
    };

    packages = eachSystem (system: {
      default = self.packages.${system}.hello;
      inherit (pkgsFor.${system}) hello;
    });
    devShells = eachSystem (system: let
      pkgs = pkgsFor.${system};
    in {
      default = pkgs.mkShell {
        nativeBuildInputs = with pkgs; [inputs.roc.packages.${system}.default];
      };
    });
  };
}
