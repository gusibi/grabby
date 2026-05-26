const esbuild = require('esbuild');

esbuild.build({
    entryPoints: ['./src/defuddle-bridge.js'],
    bundle: true,
    outfile: './lib/defuddle.bundle.js',
    format: 'iife',
    platform: 'browser',
    target: ['es2020'],
    minify: true,
}).then(() => {
    console.log('Build complete: lib/defuddle.bundle.js');
}).catch((err) => {
    console.error('Build failed:', err);
    process.exit(1);
});
