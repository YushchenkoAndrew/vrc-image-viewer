{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [ go gopls gotools golangci-lint delve gcc gnumake ];

  shellHook = ''
    YES_ANSWERS="y Y yes YES"

    export GOROOT="${pkgs.go}/share/go"
    export GOPATH=$PWD/.gopath
    export PATH=$GOPATH/bin:$PATH

    echo "Go development environment!"
  '';
}
