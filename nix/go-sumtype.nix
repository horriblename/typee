{
  source,
  version,
  buildGoModule,
}:
buildGoModule {
  name = "go-sumtype";
  inherit version;
  src = source;

  vendorHash = "sha256-EFcvb2heqBHSlRFHWaD3NT3fGhQp5BGzqAUosEiTMYY=";
}
