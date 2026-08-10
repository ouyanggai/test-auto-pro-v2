'use strict'

const fs = require('fs')
const path = require('path')
const RuntimeHealthPlugin = require('./build/runtime-health-plugin')

const projectRoot = __dirname
const sourceRoot = path.resolve(process.env.FORM_RUNTIME_SOURCE_DIR || path.join(projectRoot, 'runtime-source'))
const sourceMetadataPath = path.join(sourceRoot, '.f007-source.json')

// resolveSource 把 @ 永久绑定到本轮实际构建的完整 rsh-flow-components 源码。
function resolveSource (relativePath = '') {
  return path.join(sourceRoot, 'src', relativePath)
}

// registeredComponentNames 从真实上游 main.js 读取 FormMaking 注册名，避免本地再维护一份空或过期组件表。
function registeredComponentNames () {
  const content = fs.readFileSync(resolveSource('main.js'), 'utf8')
  return [...content.matchAll(/name:\s*['"]([^'"]+)['"]\s*,\s*component:/g)].map(match => match[1])
}

if (!fs.existsSync(sourceMetadataPath)) {
  throw new Error(`缺少已核验的 rsh-flow-components 来源快照: ${sourceMetadataPath}`)
}

process.env.VUE_APP_TARGET_COMPONENT_NAMES = JSON.stringify(registeredComponentNames())

module.exports = {
  publicPath: '/form-runtime/',
  outputDir: process.env.FORM_RUNTIME_OUT_DIR || path.resolve(projectRoot, '../web/dist/form-runtime'),
  assetsDir: './static',
  lintOnSave: false,
  productionSourceMap: false,
  pages: {
    index: {
      entry: path.join(projectRoot, 'src/main.js'),
      template: path.join(projectRoot, 'public/index.html'),
      title: '流程表单数据配置'
    }
  },
  devServer: {
    host: '127.0.0.1',
    port: 19001,
    open: false,
    hot: true
  },
  css: {
    loaderOptions: {
      sass: {
        sassOptions: { outputStyle: 'expanded' }
      }
    }
  },
  chainWebpack (config) {
    config.module.rule('svg').exclude.add(resolveSource('assets/icons')).end()
    config.module.rule('icons')
      .test(/\.svg$/)
      .include.add(resolveSource('assets/icons')).end()
      .use('svg-sprite-loader').loader('svg-sprite-loader').options({ symbolId: 'icon-[name]' }).end()
    config.plugins.delete('preload')
    config.plugins.delete('prefetch')
  },
  configureWebpack: {
    resolve: {
      alias: {
        'vue$': 'vue/dist/vue.esm.js',
        '@': resolveSource(),
        '@runtime': resolveSource(),
        '@/adapters/postMessage$': path.join(projectRoot, 'src/runtime/upstreamPostMessage.js'),
        '@/utils/runtimeAuthSync$': path.join(projectRoot, 'src/runtime/runtimeAuthSync.js'),
        '@/utils/auth$': path.join(projectRoot, 'src/runtime/memoryAuth.js'),
        '@/config/env$': path.join(projectRoot, 'src/runtime/runtimeEnvironment.js'),
        '@/views/BudgetManage': resolveSource('cross-modules/BudgetManage'),
        '@/views/PerformanceManage': resolveSource('cross-modules/PerformanceManage'),
        '@/views/BacklogManage': resolveSource('cross-modules/BacklogManage'),
        '@/views/QuestionnaireManage': resolveSource('cross-modules/QuestionnaireManage'),
        '@/views/TaskManage': resolveSource('cross-modules/TaskManage'),
        '@/views/flowLibrary': resolveSource('cross-modules/flowLibrary')
      }
    },
    plugins: [new RuntimeHealthPlugin(sourceMetadataPath)]
  }
}
