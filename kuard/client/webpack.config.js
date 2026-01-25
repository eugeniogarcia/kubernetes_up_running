const webpack = require('webpack');
const path = require('path');

const BUILD_DIR = path.resolve(__dirname, '../pkg/sitedata/built');
const APP_DIR = path.resolve(__dirname, 'src');

module.exports = {
  entry: './src/index.jsx',
  module: {
    rules: [
      {
        test: /\.(js|jsx)$/,
        exclude: /node_modules/,
        use: {
          loader: 'babel-loader',
          options: {
            presets: [
              ['@babel/preset-env', { targets: { browsers: ['last 2 versions'] } }],
              '@babel/preset-react'
            ]
          }
        }
      }
    ]
  },
  resolve: {
    extensions: ['.js', '.jsx']
  },
  output: {
    path: BUILD_DIR,
    publicPath: '/built/',
    filename: 'bundle.js',
    clean: true
  },
  cache: {
    type: 'filesystem'
  },
  performance: { hints: false },
  devServer: {
    static: [
      {
        directory: BUILD_DIR,
        publicPath: '/built/'
      }
    ],
    port: 8083,
    hot: true,
    compress: true,
    proxy: [
      {
        context: ['**'],
        target: 'http://localhost:8084',
        changeOrigin: true,
        bypass: function(req, res, proxyOptions) {
          // Don't proxy /built requests - serve from webpack
          if (req.path.startsWith('/built/')) {
            return false; // let webpack serve it
          }
        }
      }
    ]
  }
};

