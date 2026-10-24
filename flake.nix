{
  description = "VRChat image viewer and NixOS service";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs, ... }: {
    packages.x86_64-linux.default =
      let
        pkgs = nixpkgs.legacyPackages.x86_64-linux;
      in
      pkgs.buildGoModule {
        pname = "vrc-image-viewer";
        version = "master";
        src = self;
        vendorHash = pkgs.lib.fakeHash;
      };

    nixosModules.default =
      import ./nixos/module.nix {
        inherit self;
      };
  };
}
