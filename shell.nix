{ pkgs ? import <nixpkgs> {} }:

let
  # Wails v3 links GTK4 and WebKitGTK 6.0 on Linux.
  linuxLibs = with pkgs; [
    webkitgtk_6_0
    gtk4
    libsecret
    glib
  ];
in
  pkgs.mkShell {
    name = "gitura-dev";

    buildInputs = with pkgs; [
      go
      # The Taskfiles call `wails3`; run the version pinned in go.mod.
      (writeShellScriptBin "wails3" ''exec go tool wails3 "$@"'')

      # Frontend
      bun
      nodejs_24

      # Linting
      golangci-lint

      # App icon (`just icons`)
      resvg
      imagemagick
      icoutils

      # Build tooling
      pkg-config
      gcc
      git
    ] ++ pkgs.lib.optionals pkgs.stdenv.hostPlatform.isLinux linuxLibs;

    shellHook = ''
      export CGO_ENABLED=1

      echo "gitura dev shell"
      echo "  go:            $(go version)"
      echo "  bun:           $(bun --version)"
      echo "  golangci-lint: $(golangci-lint --version 2>/dev/null | head -1 || echo 'n/a')"
      echo "  webkitgtk:     $(pkg-config --modversion webkitgtk-6.0 2>/dev/null || echo 'MISSING')"
      echo ""
      echo "Commands:"
      echo "  just dev       — run with hot reload"
      echo "  go test ./...  — run all tests"
      echo "  just build     — build production binary"
    '';
  }
