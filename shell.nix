{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = with pkgs; [
    go_1_27
    python3
    luajit
    protobuf
    android-tools
    scrcpy
    jq
    tcpdump
  ];
}
