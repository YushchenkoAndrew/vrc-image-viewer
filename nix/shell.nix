{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [ go gopls gotools golangci-lint delve gcc gnumake ];

  shellHook = ''
    export GOROOT="${pkgs.go}/share/go"
    export GOPATH=$PWD/.gopath
    export PATH=$GOPATH/bin:$PATH

    echo "Opening port 3001..."
    sudo nixos-firewall-tool open tcp 3001
    echo "Go development environment!"
  '';
}
