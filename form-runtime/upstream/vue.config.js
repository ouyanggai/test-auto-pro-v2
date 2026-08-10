'use strict';
const path = require('path');

function resolve(dir) {
  return path.join(__dirname, dir);
}

module.exports = {
  publicPath: '/flow/',
  outputDir: 'dist',
  assetsDir: './static',
  lintOnSave: false,
  productionSourceMap: false,
  devServer: {
    port: 9528,
    open: true
  },
  css: {
    loaderOptions: {
      sass: {
        sassOptions: {
          outputStyle: 'expanded'
        }
      }
    }
  },
  chainWebpack(config) {
    // SVG sprite loader for icons
    config.module
      .rule('svg')
      .exclude.add(resolve('src/assets/icons'))
      .end();

    config.module
      .rule('icons')
      .test(/\.svg$/)
      .include.add(resolve('src/assets/icons'))
      .end()
      .use('svg-sprite-loader')
      .loader('svg-sprite-loader')
      .options({
        symbolId: 'icon-[name]'
      })
      .end();

    // 删除 preload 插件
    config.plugins.delete('preload');
    config.plugins.delete('prefetch');
  },
  configureWebpack: {
    resolve: {
      alias: {
        'vue$': 'vue/dist/vue.esm.js',
        '@': resolve('src'),
        // 跨模块依赖 — 将 @/views/XXX 重定向到 cross-modules/
        '@/views/BudgetManage': resolve('src/cross-modules/BudgetManage'),
        '@/views/PerformanceManage': resolve('src/cross-modules/PerformanceManage'),
        '@/views/BacklogManage': resolve('src/cross-modules/BacklogManage'),
        '@/views/QuestionnaireManage': resolve('src/cross-modules/QuestionnaireManage'),
        '@/views/TaskManage': resolve('src/cross-modules/TaskManage'),
        '@/views/flowLibrary': resolve('src/cross-modules/flowLibrary')
      }
    },
    optimization: {
      splitChunks: {
        chunks: 'all',
        cacheGroups: {
          vendor: {
            name: 'chunk-vendor',
            test: /[\\/]node_modules[\\/]/,
            priority: 10,
            chunks: 'initial'
          },
          elementUI: {
            name: 'chunk-elementUI',
            test: /[\\/]node_modules[\\/]element-ui[\\/]/,
            priority: 20
          },
          commons: {
            name: 'chunk-commons',
            test: resolve('src/components'),
            minChunks: 2,
            priority: 5,
            reuseExistingChunk: true
          }
        }
      }
    }
  }
};
