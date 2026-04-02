{
  description = "gitura — GitHub PR review desktop application";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        version = "0.1.0"; # x-release-please-version

        # Fetch frontend npm dependencies using the per-package integrity hashes
        # recorded in frontend/package-lock.json (no aggregate hash required).
        frontendDeps = pkgs.importNpmLock {
          npmRoot = ./frontend;
        };

        # Build the Vue/Vite frontend and expose the dist directory.
        frontend = pkgs.stdenv.mkDerivation {
          name = "gitura-frontend-${version}";
          src = ./frontend;

          nativeBuildInputs = [
            pkgs.nodejs
            pkgs.npmHooks.npmConfigHook
          ];

          inherit (frontendDeps) npmDeps;

          buildPhase = ''
            npm run build
          '';

          installPhase = ''
            cp -r dist $out
          '';
        };

        # Native build inputs required for CGO compilation.
        nativeBuildInputs = [ pkgs.pkg-config ]
          ++ pkgs.lib.optionals pkgs.stdenv.isLinux [
            pkgs.webkitgtk_4_1.dev
            pkgs.gtk3.dev
            pkgs.libsecret.dev
            pkgs.glib.dev
          ];

        # Runtime build inputs (native libraries linked into the binary).
        buildInputs = pkgs.lib.optionals pkgs.stdenv.isLinux [
          pkgs.webkitgtk_4_1
          pkgs.libsecret
          pkgs.gtk3
          pkgs.glib
        ];
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "gitura";
            inherit version;

            src = ./.;

            # Compute by running: nix build .#packages.<system>.default 2>&1 | grep "got:"
            vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";

            inherit nativeBuildInputs buildInputs;

            # Copy the pre-built frontend assets so //go:embed picks them up.
            preBuild = ''
              cp -r ${frontend} frontend/dist
            '';

            CGO_ENABLED = "1";

            # webkit2_41 build tag selects webkit2gtk-4.1 (matches wails.json).
            tags = [ "webkit2_41" ];

            ldflags = [ "-s" "-w" ];

            meta = with pkgs.lib; {
              description = "GitHub PR review desktop application built with Wails + Vue 3";
              homepage = "https://github.com/viicslen/gitura";
              mainProgram = "gitura";
              platforms = platforms.linux ++ platforms.darwin;
            };
          };
        };

        devShells.default = import ./shell.nix { inherit pkgs; };
      }
    );
}
