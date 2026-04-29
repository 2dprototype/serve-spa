#!/usr/bin/env node

const { Command } = require('commander');
const path = require('path');
const { startServer } = require('../lib/server');

const program = new Command();

program
    .name('serve-spa')
    .description('Serve a Single Page Application with automatic QR code generation')
    .version('1.0.0')
    .argument('[directory]', 'Directory to serve', '.')
    .option('-p, --port <number>', 'Port to listen on', '5600')
    .option('-h, --host <string>', 'Host to bind to', '0.0.0.0')
    .option('-i, --index <file>', 'SPA entry point file', 'index.html')
    .option('-s, --static-dir <dir>', 'Static files directory (relative to source)', '')
    .option('--no-qr', 'Disable QR code generation')
    .option('--no-open', 'Disable auto-opening browser')
    .option('--cors', 'Enable CORS for all origins')
    .option('-b, --base <path>', 'Base path for the SPA', '/')
    .parse(process.argv);

const options = program.opts();
const sourceDir = program.args[0] || '.';

// Convert relative path to absolute
const absolutePath = path.resolve(process.cwd(), sourceDir);

// Start the server
startServer({
    sourceDir: absolutePath,
    port: parseInt(options.port),
    host: options.host,
    indexFile: options.index,
    staticDir: options.staticDir,
    showQr: options.qr,
    openBrowser: options.open,
    enableCors: options.cors,
    basePath: options.base,
    onReady: (url, ipUrl) => {
        console.log('\nSPA is ready!');
        console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━');
        console.log(`   Local:    ${url}`);
        if (ipUrl) {
            console.log(`   Network:  ${ipUrl}`);
        }
        console.log(`   Serving:  ${absolutePath}`);
        console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');
        console.log('Press Ctrl+C to stop the server\n');
    }
});