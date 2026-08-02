# JimmyVirus Two

A remote command and control system using Cloudflare tunneling and GitHub gists for URL distribution. No vibecoding used (except the readme markdown).

## Overview

**Server**: Starts a local HTTP server, creates a Cloudflare tunnel, and updates a GitHub gist with the tunnel URL.

**Client**: Fetches the tunnel URL from the gist and connects to execute remote commands (screenshots, file uploads, etc.).

## Requirements

- Go 1.16+
- GitHub personal access token (for gist updates)
- Internet connection (cloudflared creates free quick tunnels automatically)

## Setup

### Server

1. Create a `.env` file in the server directory:
```
TOKEN=your_github_token_here
```

2. Update the gist ID in `updateGist()` function in `server/main.go`:
```go
"https://api.github.com/gists/YOUR_GIST_ID"
```

3. Create a GitHub gist with a file `tunnel.txt` (can be empty initially)

### Client

Update the gist URL in `fetchTunnelURL()` in `client/main.go`:
```go
"https://gist.githubusercontent.com/YOUR_USERNAME/YOUR_GIST_ID/raw/tunnel.txt"
```

## Compilation

### Linux/Mac
```bash
cd server && go build -o ../server ./main.go && cd ..
cd client && go build -o ../client ./main.go && cd ..
```

### Windows (from Linux/Mac)
```bash
# Server (no console window)
cd server && GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o ../server.exe ./main.go && cd ..

# Client (no console window)
cd client && GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o ../client.exe ./main.go && cd ..
```

## Usage

### Start the Server
```bash
./server
```
The server will:
1. Download cloudflared if not present
2. Create a tunnel automatically (free, no account needed)
3. Extract the tunnel URL
4. Update the GitHub gist with the URL
5. Listen for incoming commands

### Start the Client
```bash
./client
```
The client will:
1. Fetch the tunnel URL from the gist
2. Connect to the server through the tunnel
3. Wait for commands to execute

### Send Commands

From the server's stdin, type:
- `ss 5` — Capture 5 screenshots (saved as `./ss.webp` on server)
- `idle` — Do nothing/reset

## Architecture

```
Client ──(HTTPS Tunnel)──→ Cloudflare ──→ Server localhost:10000
  ↑                                            ↑
  └─── GitHub Gist (fetch URL) ← (update) ───┘
```

## How It Works

1. **Server** starts and creates cloudflared tunnel (automatically, no account needed)
2. **Server** captures tunnel URL using regex pattern matching
3. **Server** updates GitHub gist with tunnel URL
4. **Client** fetches tunnel URL from gist
5. **Client** connects through tunnel to server's public endpoint
6. **Server** sends commands via `/get-command` endpoint
7. **Client** executes commands and uploads results via `/upload`

## Project Structure

```
jimmyvirustwo/
├── server/
│   ├── main.go
│   ├── cloudflared (auto-downloaded)
│   └── .env (GitHub token)
├── client/
│   └── main.go
└── README.md
```

## Security Warning

⚠️ This is a demonstration project for educational purposes only. Use responsibly and only on systems you own or have permission to test on.

## Notes

- Cloudflare quick tunnels are free and don't require an account
- The tunnel URL is temporary and changes each time the server restarts
- Screenshots are uploaded as WebP format
- All communication is encrypted through Cloudflare's tunnel
