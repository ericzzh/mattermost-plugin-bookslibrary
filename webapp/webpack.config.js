const exec = require('child_process').exec;

const path = require('path');

const PLUGIN_ID = require('../plugin.json').id;

const NPM_TARGET = process.env.npm_lifecycle_event; //eslint-disable-line no-process-env
let mode = 'production';
let devtool = 'source-map';
if (NPM_TARGET === 'debug' || NPM_TARGET === 'debug:watch'|| NPM_TARGET === 'dev-server') {
    mode = 'development';
    devtool = 'source-map';
}

const plugins = [];
if (NPM_TARGET === 'build:watch' || NPM_TARGET === 'debug:watch' || NPM_TARGET === 'dev-server') {
    plugins.push({
        apply: (compiler) => {
            compiler.hooks.watchRun.tap('WatchStartPlugin', () => {
                // eslint-disable-next-line no-console
                console.log('Change detected. Rebuilding webapp.');
            });
            compiler.hooks.afterEmit.tap('AfterEmitPlugin', () => {
                exec('cd .. && make deploy-from-watch', (err, stdout, stderr) => {
                    if (stdout) {
                        process.stdout.write(stdout);
                    }
                    if (stderr) {
                        process.stderr.write(stderr);
                    }
                });
            });
        },
    });
}

let config = {
    entry: [
        './src/index.tsx',
    ],
    resolve: {
        modules: [
            'src',
            'node_modules',
            path.resolve(__dirname),
        ],
        extensions: ['*', '.js', '.jsx', '.ts', '.tsx'],
    },
    module: {
        rules: [
            {
                test: /\.(js|jsx|ts|tsx)$/,
                exclude: /node_modules/,
                use: {
                    loader: 'babel-loader',
                    options: {
                        cacheDirectory: true,

                        // Babel configuration is in babel.config.js because jest requires it to be there.
                    },
                },
            },
            {
                test: /\.(scss|css)$/,
                use: [
                    'style-loader',
                    {
                        loader: 'css-loader',
                    },
                    {
                        loader: 'sass-loader',
                        options: {
                            sassOptions: {
                                includePaths: ['node_modules/compass-mixins/lib', 'sass'],
                            },
                        },
                    },
                ],
            },
        ],
    },
    externals: {
        react: 'React',
        redux: 'Redux',
        'react-redux': 'ReactRedux',
        'prop-types': 'PropTypes',
        'react-bootstrap': 'ReactBootstrap',
        'react-router-dom': 'ReactRouterDom',
    },
    output: {
        devtoolNamespace: PLUGIN_ID,
        path: path.join(__dirname, '/dist'),
        publicPath: '/',
        filename: 'main.js',
    },
    devtool,
    mode,
    plugins,
};

if (NPM_TARGET === 'dev-server') {
    config = {
        ...config,
        devServer: {
            hot: true,
            injectHot: true,
            liveReload: false,
            overlay: false,
            proxy: [{
                context: () => true,
                bypass(req) {
                    if (req.url.indexOf('/static/plugins/com.github.ericzzh.mattermost-plugin-bookslibrary') === 0) {
                        return '/main.js'; // return the webpacked asset
                    }
                    return null;
                },
                logLevel: 'debug',
                target: 'http://localhost:8065',
                xfwd: true,
                ws: true,
            }],
            port: 9005,
            watchContentBase: true,
            writeToDisk: false,
        },
        performance: false,
        optimization: {
            ...config.optimization,
            splitChunks: false,
        },
    };
}

module.exports = config;

