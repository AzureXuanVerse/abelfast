{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = with pkgs; [
    go_1_27
    python3
    luajit
    protobuf
    sqlc
    android-tools
    scrcpy
    jq
    tcpdump
  ];
}
