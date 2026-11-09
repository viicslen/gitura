set dotenv-load

client_id := env("GITHUB_CLIENT_ID", "")

# Copy .env.example to .env (run this once after cloning).
init:
    #!/usr/bin/env sh
    if [ -f .env ]; then
        echo ".env already exists, skipping."
    else
        cp .env.example .env
        echo "Created .env — fill in your GITHUB_CLIENT_ID."
    fi

# Start the development server with hot reload.
dev client_id=client_id:
    wails dev -ldflags "-X 'main.githubClientID={{ client_id }}'"

# Build the app for the current platform. Requires GITHUB_CLIENT_ID.
build client_id=client_id:
    #!/usr/bin/env sh
    if [ -z "{{ client_id }}" ]; then
        echo "Error: client_id is required. Pass it as an argument or set GITHUB_CLIENT_ID."
        exit 1
    fi
    wails build -ldflags "-X 'main.githubClientID={{ client_id }}'"

# Regenerate the app icons from frontend/src/assets/images/logo.svg.
icons:
    #!/usr/bin/env sh
    set -eu
    src=frontend/src/assets/images/logo.svg
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    # appicon.png keeps the transparent 100px margin macOS icons need; the Windows sizes are full-bleed.
    resvg -w 824 "$src" "$tmp/824.png"
    magick "$tmp/824.png" -background none -gravity center -extent 1024x1024 build/appicon.png
    for s in 16 24 32 48 64 128 256; do resvg -w "$s" "$src" "$tmp/$s.png"; done
    icotool -c -o build/windows/icon.ico "$tmp/16.png" "$tmp/24.png" "$tmp/32.png" "$tmp/48.png" "$tmp/64.png" "$tmp/128.png" -r "$tmp/256.png"
