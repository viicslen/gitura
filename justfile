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
