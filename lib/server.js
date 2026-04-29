const express = require('express');
const path = require('path');
const os = require('os');
const dns = require('dns');
const qrcode = require('qrcode-terminal');
const fs = require('fs');

/**
 * Start the SPA server
 * @param {Object} config - Server configuration
 * @param {string} config.sourceDir - Directory to serve
 * @param {number} config.port - Port number
 * @param {string} config.host - Host to bind to
 * @param {string} config.indexFile - SPA entry point file
 * @param {string} config.staticDir - Static files directory
 * @param {boolean} config.showQr - Show QR code
 * @param {boolean} config.openBrowser - Auto-open browser
 * @param {boolean} config.enableCors - Enable CORS
 * @param {string} config.basePath - Base path for the SPA
 * @param {Function} config.onReady - Callback when server is ready
 */
function startServer(config) {
    const {
        sourceDir,
        port = 5600,
        host = '0.0.0.0',
        indexFile = 'index.html',
        staticDir = '',
        showQr = true,
        openBrowser = true,
        enableCors = false,
        basePath = '/',
        onReady = null
    } = config;

    // Validate source directory
    if (!fs.existsSync(sourceDir)) {
        console.error(`Error: Directory "${sourceDir}" does not exist`);
        process.exit(1);
    }

    const app = express();

    // Enable CORS if requested
    if (enableCors) {
        app.use((req, res, next) => {
            res.header('Access-Control-Allow-Origin', '*');
            res.header('Access-Control-Allow-Headers', 'Origin, X-Requested-With, Content-Type, Accept');
            next();
        });
    }

    // Auto-detect common static directories
    if (!staticDir) {
        const commonDirs = ['res', 'fonts', 'css', 'js', 'lib', 'assets', 'static', 'public'];
        commonDirs.forEach(dir => {
            const dirPath = path.resolve(sourceDir, dir);
            if (fs.existsSync(dirPath)) {
                app.use(`/${dir}`, express.static(dirPath));
                console.log(`Serving static directory: /${dir}`);
            }
        });
    } else {
        // Use specified static directory
        const staticPath = path.resolve(sourceDir, staticDir);
        if (fs.existsSync(staticPath)) {
            app.use(express.static(staticPath));
            console.log(`Serving static directory: ${staticPath}`);
        }
    }

    // Serve SPA - all routes go to index.html
    const indexPath = path.resolve(sourceDir, indexFile);

    if (!fs.existsSync(indexPath)) {
        console.error(`Error: Index file "${indexFile}" not found in ${sourceDir}`);
        process.exit(1);
    }

    // Normalize base path
    const normalizedBasePath = basePath === '/' ? '' : basePath;

    // Handle base path
    app.get(`${normalizedBasePath}*`, (req, res) => {
        res.sendFile(indexPath);
    });

    // Start server
    const server = app.listen(port, host, () => {
        const localUrl = `http://localhost:${port}`;

        getPublicIPAddress((err, ip) => {
            let networkUrl = null;

            if (!err && ip) {
                networkUrl = `http://${ip}:${port}`;
            }

            // Display QR code if enabled
            if (showQr && networkUrl) {
                console.log('\nScan QR code to open on mobile:');
                qrcode.generate(networkUrl, {
                    small: true
                }, (qrCodeASCII) => {
                    console.log(qrCodeASCII);

                    if (onReady) {
                        onReady(localUrl, networkUrl);
                    }
                });
            } else {
                if (onReady) {
                    onReady(localUrl, networkUrl);
                }
            }

            // Auto-open browser
            if (openBrowser) {
                const {
                    exec
                } = require('child_process');
                const platform = process.platform;
                let command;

                if (platform === 'win32') {
                    command = `start ${localUrl}`;
                } else if (platform === 'darwin') {
                    command = `open ${localUrl}`;
                } else {
                    command = `xdg-open ${localUrl}`;
                }

                exec(command, (error) => {
                    if (error) {
                        // Silently fail if browser can't be opened
                    }
                });
            }
        });
    });

    // Graceful shutdown
    process.on('SIGINT', () => {
        console.log('\n\nShutting down server...');
        server.close(() => {
            console.log('Server stopped.');
            process.exit(0);
        });
    });

    process.on('SIGTERM', () => {
        console.log('\n\nShutting down server...');
        server.close(() => {
            console.log('Server stopped.');
            process.exit(0);
        });
    });

    return server;
}

/**
 * Get the public IP address of the machine
 * @param {Function} callback - Callback function
 */
function getPublicIPAddress(callback) {
    const interfaces = os.networkInterfaces();
    let ipAddress = '';

    for (const interfaceName in interfaces) {
        interfaces[interfaceName].forEach((iface) => {
            // Skip internal and non-IPv4 addresses
            if (iface.family === 'IPv4' && !iface.internal) {
                // Prefer 192.168.x.x or 10.x.x.x addresses
                if (iface.address.startsWith('192.168.') ||
                    iface.address.startsWith('10.') ||
                    iface.address.startsWith('172.')) {
                    if (!ipAddress || iface.address.startsWith('192.168.')) {
                        ipAddress = iface.address;
                    }
                }
            }
        });
    }

    if (!ipAddress) {
        callback('No public IP address found.');
        return;
    }

    // Verify the IP is valid
    dns.lookup(ipAddress, (err, hostname) => {
        if (err) {
            callback(err);
        } else {
            callback(null, ipAddress);
        }
    });
}

module.exports = {
    startServer,
    getPublicIPAddress
};