{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  name = "gitura-dev";

  buildInputs = with pkgs; [
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
    # (set via GOFLAGS in shellHook) so it links against 4.1 directly.
    webkitgtk_4_1
    libsecret
    gtk3
    glib

    # Build tooling
    pkg-config
    gcc

    # Useful dev utilities
    git
  ];

  shellHook = ''
    export CGO_ENABLED=1

    # Wails v2 CGO uses webkit2gtk-4.0 by default. The webkit2_41 build tag
    # (set in wails.json "build:tags") switches it to webkit2gtk-4.1.
    # GOFLAGS covers direct `go build` invocations outside of wails.
    export GOFLAGS="-tags=webkit2_41"

    # Primary pkg-config paths from Nix packages
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
