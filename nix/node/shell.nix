{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  # Build-time tools (executables needed on the host to compile the code)
  nativeBuildInputs = with pkgs; [
    pkg-config
    gcc
    gnumake
    python3
  ];

  # Runtime dependencies and libraries to link against
  buildInputs = with pkgs; [
    nodejs_24

    # Graphics / UI libs
    cairo
    pango
    librsvg

    # Fonts / text rendering
    freetype
    harfbuzz
    glib

    # Image libs
    libjpeg
    giflib
    libpng
    zlib

    # System libs
    util-linux    # libuuid
  ];

  # Fix for runtime libraries (libuuid, etc.)
  env = {
    AWS_CONFIG_FILE = "./.aws/config";
    AWS_SHARED_CREDENTIALS_FILE = "./.aws/credentials";
    NODE_OPTIONS = "--max-old-space-size=8192";
    
    # pkg-config reads PKG_CONFIG_PATH to resolve CFLAGS and LIBS dynamically
    PKG_CONFIG_PATH = with pkgs; lib.makeSearchPath "lib/pkgconfig" [
      cairo
      pango
      librsvg
      freetype
      harfbuzz
      glib
      libjpeg
      giflib
      libpng
      zlib
      util-linux
    ];

    LD_LIBRARY_PATH = with pkgs; lib.makeLibraryPath [
      cairo
      pango
      librsvg
      freetype
      harfbuzz
      glib
      libjpeg
      giflib
      libpng
      zlib
      util-linux
    ];
  };
}
