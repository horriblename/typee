{
  libgccjit,
  glibc,
  libgcc,
  stdenv,
  gcc,
  ninja,
  meson,
}:
stdenv.mkDerivation {
  pname = "testing";
  version = "0.1";
  src = ./.;

  nativeBuildInputs = [gcc meson ninja libgccjit glibc libgcc];
  buildInputs = [glibc libgccjit gcc libgcc];

  # buildPhase = ''
  #   $CC ./test.c -o testy
  # '';
  # installPhase = ''
  #   cp testy $out/testy
  # '';
}
