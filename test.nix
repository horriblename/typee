{
   meson,
   ninja,
   pkg-config,
   libgccjit,
   glibc,
   libgcc,
   gcc,
   stdenv,
}: stdenv.mkDerivation {
   pname = "testing";
   version = "0.1";

   src = ./.;

   nativeBuildInputs = [gcc meson ninja pkg-config];
   buildInputs = [glibc libgcc libgccjit];

   buildPhase = ''
      gcc test.c -o $out/testy
   '';
}
