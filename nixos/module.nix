{ config, pkgs, lib, ... }:

let
  cfg = config.services.vrc-image-viewer;
  server = self.packages.${pkgs.system}.default;

  yamlFormat = pkgs.formats.yaml { };

  options = yamlFormat.generate "config.yaml" {
    server = { address = cfg.address; prefix = cfg.prefix; };
    vrchat = { src = cfg.src; dst = cfg.dst; };
  };
in
{
  options.services.vrc-image-viewer = {
    enable = lib.mkEnableOption "VRChat image viewer";

    address = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1:3001";
      description = "Address and port on which the server listens.";
    };

    prefix = lib.mkOption {
      type = lib.types.str;
      default = "/";
      description = "HTTP URL prefix.";
    };

    src = lib.mkOption {
      type = lib.types.str;
      default = "/home/mortis/.steam/steam/steamapps/compatdata/438100/pfx/drive_c/users/steamuser/Pictures/VRChat";
      description = "Directory containing the original VRChat images.";
    };

    dst = lib.mkOption {
      type = lib.types.str;
      default = "/home/mortis/Pictures/VRChat-edited";
      description = "Directory where processed images are stored.";
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.vrc-image-viewer = {
      description = "VRChat Image Viewer";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" ];

      serviceConfig = {
        Type = "simple";
        User = "mortis";
        Group = "users";

        ExecStart = "${server}/bin/vrc-image-viewer --config ${options}";

        Restart = "on-failure";
        RestartSec = "3s";

        ReadWritePaths = [ cfg.destinationDirectory ];

        NoNewPrivileges = true;
        PrivateTmp = true;
      };
    };
  };
}