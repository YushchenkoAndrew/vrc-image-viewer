{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [ go gopls gotools golangci-lint delve gcc gnumake ];

  shellHook = ''
    YES_ANSWERS="y Y yes YES"

    export GOROOT="${pkgs.go}/share/go"
    export GOPATH=$PWD/.gopath
    export PATH=$GOPATH/bin:$PATH

    if ! ss -tulpn | grep -q ":3001 "; then
      echo "⚠  Network ports not initialized"
      read -p "   Open required ports? (yes/no) " CONFIRM
      if echo "$YES_ANSWERS" | grep -qw "$CONFIRM"; then
        echo ""
        echo "🔧 Opening ports..."
        sudo nixos-firewall-tool open tcp 3001 && echo "   ✅ 3001/tcp — vr-sync"
        sudo nixos-firewall-tool open udp 6969 && echo "   ✅ 6969/udp — SlimeVR"
        echo ""
        echo "✔  Network initialization complete"
      else
        echo "✖  Skipped — some features may not work"
      fi
    fi

    echo "Go development environment!"
  '';
}
