{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  # Packages you need for building/running
  buildInputs = with pkgs; [];
  shellHook = ''
    echo "Opening port 3001..."
    sudo nixos-firewall-tool open tcp 3001
  '';
}
