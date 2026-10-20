{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = with pkgs; [
    go_1_27
    python3
    uv
    luajit
    protobuf
    sqlc
    android-tools
    scrcpy
    jq
    tcpdump
  ];

  # Camoufox (the wiki reader in internal/tools) downloads a prebuilt Firefox,
  # which runs through nix-ld with these libraries.
  NIX_LD_LIBRARY_PATH = pkgs.lib.makeLibraryPath (with pkgs; [
    stdenv.cc.cc
    zlib
    glib
    gtk3
    dbus-glib
    dbus
    pango
    cairo
    atk
    gdk-pixbuf
    freetype
    fontconfig
    alsa-lib
    libdrm
    libgbm
    libGL
    pciutils
    libx11
    libxcb
    libxcomposite
    libxcursor
    libxdamage
    libxext
    libxfixes
    libxi
    libxrandr
    libxrender
    libxtst
  ]);
}
