# serve-spa

Quickly serve a Single Page Application with QR code for mobile testing.

## Installation

```bash
npm install -g serve-spa
```

## Usage

```bash
# Serve current directory
serve-spa

# Serve specific directory
serve-spa ./dist

# Use custom port
serve-spa -p 3000
```

## Options

```
-p, --port <number>    Port to use (default: 5600)
-h, --host <string>    Host to bind (default: 0.0.0.0)
-i, --index <file>     Entry point file (default: index.html)
--no-qr                Disable QR code
--no-open              Don't open browser
--cors                 Enable CORS
-b, --base <path>      Base path (default: /)
```

## Examples

```bash
# Serve React build on port 8080
serve-spa ./build -p 8080

# Serve without QR code
serve-spa ./dist --no-qr

# Enable CORS
serve-spa ./public --cors
```

## How It Works

- Automatically detects and serves static folders (css, js, images, etc.)
- All other routes redirect to your SPA's index.html
- Generates QR code for easy mobile device testing
- Shows both local and network URLs