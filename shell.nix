{ pkgs ? import <nixpkgs> {} }:

let
  devPackages = with pkgs; [
    # Go toolchain
    go_1_25

    # Wails CLI (v2)
    wails

    # Frontend
    nodejs_22

    # Linting
    golangci-lint

    # Wails Linux runtime deps: WebKit2GTK and secret service
    # Wails v2 defaults to webkit2gtk-4.0; we use the webkit2_41 Go build tag
    # (set via GOFLAGS below) so it links against 4.1 directly.
    webkitgtk_4_1
    webkitgtk_4_1.dev
    libsecret
    libsecret.dev
    gtk3
    gtk3.dev
    glib
    glib.dev

    # Build tooling
    pkg-config
    gcc

    # Useful dev utilities
    git
  ];

  # Minimal env vars needed inside the FHS namespace for CGO compilation.
  # Intentionally no echo messages — this profile runs on every wails call.
  fhsProfile = ''
    export CGO_ENABLED=1
    export GOFLAGS="-tags=webkit2_41"
    export PKG_CONFIG_PATH="${pkgs.webkitgtk_4_1.dev}/lib/pkgconfig:${pkgs.gtk3.dev}/lib/pkgconfig:${pkgs.libsecret.dev}/lib/pkgconfig:${pkgs.glib.dev}/lib/pkgconfig''${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
  '';

  # On NixOS/Linux, wrap the wails binary in a minimal FHS environment so it
  # (and the CGO subprocesses it spawns) can find libraries at standard paths
  # (/usr/lib, /usr/include, etc.) instead of relying on Nix store paths.
  #
  # Named "wails" so ${fhsWails}/bin/wails transparently replaces the raw
  # binary when prepended to PATH in the shellHook below.
  fhsWails = pkgs.buildFHSEnv {
    name = "wails";
    targetPkgs = _: devPackages;
    profile = fhsProfile;
    runScript = "wails";
  };

in
  pkgs.mkShell {
    name = "gitura-dev";

    buildInputs = devPackages;

    shellHook = ''
      ${pkgs.lib.optionalString pkgs.stdenv.isLinux ''
        # Prepend the FHS-wrapped wails so all invocations — including
        # `just dev` and `just build` — run inside the FHS environment.
        # The raw wails from devPackages remains available inside the FHS
        # namespace (via targetPkgs), but is shadowed here in the outer shell.
        export PATH="${fhsWails}/bin:$PATH"
      ''}

      export CGO_ENABLED=1
      export GOFLAGS="-tags=webkit2_41"
      export PKG_CONFIG_PATH="${pkgs.webkitgtk_4_1.dev}/lib/pkgconfig:${pkgs.gtk3.dev}/lib/pkgconfig:${pkgs.libsecret.dev}/lib/pkgconfig:${pkgs.glib.dev}/lib/pkgconfig''${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"

      echo "gitura dev shell"
      echo "  go:           $(go version)"
      echo "  node:         $(node --version)"
      echo "  wails:        $(wails version 2>/dev/null || echo 'n/a')"
      echo "  golangci-lint: $(golangci-lint --version 2>/dev/null | head -1 || echo 'n/a')"
      echo "  webkit2gtk:   $(pkg-config --modversion webkit2gtk-4.1 2>/dev/null || echo 'MISSING')"
      echo ""
      echo "Required env var:"
      echo "  export GITURA_GITHUB_CLIENT_ID=<your OAuth App client ID>"
      echo ""
      echo "Commands:"
      echo "  wails dev      — run with hot reload"
      echo "  go test ./...  — run all tests"
      echo "  wails build    — build production binary"
    '';
  }
