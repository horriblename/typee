{
  source,
  version,
  buildGoModule,
}:
buildGoModule {
  name = "go-sumtype";
  inherit version;
  src = source;

  vendorHash = "sha256-H/xcidXJdc+ahuJ2+2yinFMC8WlNVIm69qMh5gojlFw=";
}
