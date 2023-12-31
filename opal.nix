{
  ocaml,
  findlib,
  stdenv,
  source,
}:
stdenv.mkDerivation {
  pname = "opal";
  version = "0.1.1";
  src = source;

  nativeBuildInputs = [ocaml findlib];

  installPhase = ''
    runHook preInstall

    mkdir -p $out/lib/ocaml/${ocaml.version}/site-lib
    make libinstall

    runHook postInstall
  '';
}
