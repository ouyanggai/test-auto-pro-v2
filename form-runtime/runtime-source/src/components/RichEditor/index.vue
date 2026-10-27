<!--
 * @Author: junshao
 * @Date: 2022-12-07 10:15:03
 * @LastEditors: oygdeMac-mini-6.local
 * @LastEditTime: 2026-08-10 11:15:13
 * @Description: file content
-->
<template>
  <div class="editor">
    <div
      ref="myeditor"
      class="rich-editor-ins"
    >
      <Toolbar
        v-if="!hideToolbar"
        style="border-bottom: 1px solid #ccc"
        :editor="editor"
        :defaultConfig="toolbarConfig"
        :mode="mode"
      />
      <Editor
        class="my-eidtor"
        style="height: 300px; overflow-y: auto"
        ref="editor"
        v-model="contentHtml"
        :defaultConfig="editorConfig"
        :mode="mode"
        @onCreated="onCreated"
        @onChange="editorChange"
      />
    </div>
    <ImageViewer
      v-if="imagePreviewVisible"
      class="rich-editor-image-viewer"
      :url-list="imagePreviewList"
      :initial-index="imagePreviewIndex"
      :append-to-body="true"
      :z-index="100000"
      :on-close="closeImagePreview"
      :on-switch="handleImagePreviewSwitch"
      @hook:mounted="handleImageViewerMounted"
      @hook:destroyed="handleImageViewerDestroyed"
    />
    <!-- <el-button type="" @click="getContent">点击获取编辑器内容</el-button> -->
    <!-- <el-button type="" @click="getConfig">点击获取工具栏配置</el-button> -->
  </div>
</template>

<script>
import Api from '@/api';
// import VueEditor from "vue2-editor";
// import { VueEditor, Quill } from "vue2-editor";
import { DomEditor } from '@wangeditor/editor';
import { Editor, Toolbar } from '@wangeditor/editor-for-vue';
import ImageViewer from 'element-ui/packages/image/src/image-viewer';
/**
 * 工具栏实例：DomEditor.getToolbar(editor)
 * 需要排除的工具：toolbar.excludeKeys
 * 获取编辑器所有菜单: editor.getAllMenuKeys()
 * 菜单默认配置：editor.getMenuConfig(key)
 */
export default {
  components: { Editor, Toolbar, ImageViewer },
  props: {
    readonly: {
      type: Boolean,
      default: false
    },
    hideToolbar: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      contentHtml: '',
      editor: null,
      imagePreviewVisible: false,
      imagePreviewList: [],
      imagePreviewIndex: 0,
      imagePreviewTimer: null,
      imagePreviewDebugId: `rich-editor-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      toolbarConfig: {
        excludeKeys: [
          '|',
          'blockquote',
          'group-video',
          // 'insertImage',
          'insertTable',
          'codeBlock',
          'fullScreen'
          // 'group-image'
        ]
      },
      editorConfig: {
        placeholder: '请输入内容...',
        readOnly: this.readonly,
        MENU_CONF: {
          uploadImage: {
            // server: Api.user.uploadFile,
            customUpload: this.uploadImageAction,
            fieldname: 'file',
            // 最多可上传几个文件，默认为 100
            maxNumberOfFiles: 10
          }
        }
      },
      mode: 'default' // or 'simple'
    };
  },
  watch: {
    readonly: {
      handler(val) {
        if (this.editor) {
          if (val) {
            this.editor.disable();
          } else {
            this.editor.enable();
          }
        }
      },
      immediate: true
    },
    contentHtml: {
      handler(newVal) {
        // 当contentHtml变化时触发change事件
        this.$emit('change', newVal);
      }
    },
    imagePreviewVisible(newVal, oldVal) {
      this.logImagePreview('visible 状态变化', {
        oldVal,
        newVal,
        listLength: this.imagePreviewList.length,
        index: this.imagePreviewIndex
      });
      this.$nextTick(() => {
        this.inspectImagePreviewViewer('visible 状态变化后的 DOM');
      });
    }
  },
  mounted() {
    document.addEventListener('mousedown', this.previewImage, true);
    this.logImagePreview('组件 mounted，已注册 document mousedown capture', {
      rootConnected: Boolean(this.$refs.myeditor && this.$refs.myeditor.isConnected),
      rootImageCount: this.$refs.myeditor
        ? this.$refs.myeditor.querySelectorAll('img').length
        : -1
    });
  },
  methods: {
    describeImagePreviewElement(element) {
      if (!element) return null;
      const rect = typeof element.getBoundingClientRect === 'function'
        ? element.getBoundingClientRect()
        : null;
      return {
        tagName: element.tagName || '',
        className: typeof element.className === 'string' ? element.className : '',
        src: element.currentSrc ||
          (element.getAttribute && element.getAttribute('src')) || '',
        isConnected: Boolean(element.isConnected),
        rect: rect
          ? {
            left: rect.left,
            top: rect.top,
            right: rect.right,
            bottom: rect.bottom,
            width: rect.width,
            height: rect.height
          }
          : null,
        html: element.outerHTML ? element.outerHTML.slice(0, 500) : ''
      };
    },
    logImagePreview(step, detail = {}) {
      const record = {
        time: new Date().toISOString(),
        id: this.imagePreviewDebugId,
        step,
        detail
      };
      window.__RICH_EDITOR_IMAGE_PREVIEW_LOGS__ =
        window.__RICH_EDITOR_IMAGE_PREVIEW_LOGS__ || [];
      window.__RICH_EDITOR_IMAGE_PREVIEW_LOGS__.push(record);
      if (window.__RICH_EDITOR_IMAGE_PREVIEW_LOGS__.length > 200) {
        window.__RICH_EDITOR_IMAGE_PREVIEW_LOGS__.shift();
      }
      console.log(
        `[RichEditor图片预览][${this.imagePreviewDebugId}] ${step}`,
        detail
      );
    },
    inspectImagePreviewViewer(step) {
      const viewers = Array.from(
        document.querySelectorAll('.rich-editor-image-viewer')
      );
      this.logImagePreview(step, {
        viewerCount: viewers.length,
        bodyChildCount: document.body.children.length,
        viewers: viewers.map(viewer => {
          const style = window.getComputedStyle(viewer);
          return {
            parentTag: viewer.parentElement && viewer.parentElement.tagName,
            display: style.display,
            visibility: style.visibility,
            opacity: style.opacity,
            position: style.position,
            zIndex: style.zIndex,
            element: this.describeImagePreviewElement(viewer)
          };
        })
      });
    },
    previewImage(event) {
      const root = this.$refs.myeditor;
      const targetImage = event.target && event.target.closest
        ? event.target.closest('img')
        : null;

      this.logImagePreview('收到 document mousedown', {
        eventType: event.type,
        button: event.button,
        clientX: event.clientX,
        clientY: event.clientY,
        visible: this.imagePreviewVisible,
        hasRoot: Boolean(root),
        rootConnected: Boolean(root && root.isConnected),
        rootContainsTarget: Boolean(root && targetImage && root.contains(targetImage)),
        eventTarget: this.describeImagePreviewElement(event.target),
        targetImage: this.describeImagePreviewElement(targetImage)
      });

      if (this.imagePreviewVisible) {
        this.logImagePreview('return：预览已经打开');
        return;
      }
      if (!root) {
        this.logImagePreview('return：myeditor ref 不存在');
        return;
      }
      if (!targetImage) {
        this.logImagePreview('return：事件目标不是图片');
        return;
      }
      if (!root.contains(targetImage)) {
        this.logImagePreview('return：图片不属于当前 RichEditor 实例');
        return;
      }

      const images = Array.from(root.querySelectorAll('img'))
        .filter(image => image.currentSrc || image.getAttribute('src'));
      this.logImagePreview('当前 RichEditor 图片查询结果', {
        imageCount: images.length,
        images: images.map((image, index) => ({
          index,
          isTarget: image === targetImage,
          element: this.describeImagePreviewElement(image)
        }))
      });
      if (!images.length) {
        this.logImagePreview('return：当前 RichEditor 没有带地址的图片');
        return;
      }

      const imagePreviewIndex = images.indexOf(targetImage);
      if (imagePreviewIndex < 0) {
        this.logImagePreview('return：目标图片不在查询结果中');
        return;
      }

      event.preventDefault();
      event.stopPropagation();

      const imagePreviewList = images.map(image =>
        image.currentSrc || image.getAttribute('src') || image.src
      );
      this.logImagePreview('准备打开预览', {
        imagePreviewIndex,
        imagePreviewList,
        defaultPrevented: event.defaultPrevented
      });
      clearTimeout(this.imagePreviewTimer);
      this.imagePreviewTimer = setTimeout(() => {
        this.logImagePreview('打开预览的 timer 已执行', {
          isDestroyed: Boolean(this._isDestroyed),
          isBeingDestroyed: Boolean(this._isBeingDestroyed)
        });
        if (this._isDestroyed || this._isBeingDestroyed) {
          this.logImagePreview('return：timer 执行时组件已经销毁');
          return;
        }
        this.imagePreviewList = imagePreviewList;
        this.imagePreviewIndex = imagePreviewIndex;
        this.imagePreviewVisible = true;
        this.logImagePreview('已写入预览状态', {
          visible: this.imagePreviewVisible,
          listLength: this.imagePreviewList.length,
          index: this.imagePreviewIndex
        });
      }, 0);
    },
    closeImagePreview() {
      this.logImagePreview('收到 ImageViewer onClose', {
        visible: this.imagePreviewVisible
      });
      this.imagePreviewVisible = false;
    },
    handleImagePreviewSwitch(index) {
      this.logImagePreview('ImageViewer 切换图片', { index });
    },
    handleImageViewerMounted() {
      this.logImagePreview('ImageViewer mounted');
      this.$nextTick(() => {
        this.inspectImagePreviewViewer('ImageViewer mounted 后的 DOM');
      });
    },
    handleImageViewerDestroyed() {
      this.logImagePreview('ImageViewer destroyed');
      this.$nextTick(() => {
        this.inspectImagePreviewViewer('ImageViewer destroyed 后的 DOM');
      });
    },
    // 编辑器创建后的回调
    onCreated(editor) {
      // 获取工具栏所有工具
      // console.log(editor.getAllMenuKeys());
      this.editor = Object.seal(editor); // 一定要用 Object.seal() ，否则会报错
      this.logImagePreview('wangEditor onCreated', {
        hasEditableContainer: typeof editor.getEditableContainer === 'function',
        htmlLength: editor.getHtml().length
      });
    },
    getContent() {
      console.log(typeof this.contentHtml, this.contentHtml);
    },
    getConfig() {
      const toolbar = DomEditor.getToolbar(this.editor);
      const curToolbarConfig = toolbar.getConfig();
      console.log('aaaa', curToolbarConfig.toolbarKeys); // 当前菜单排序和分组
    },
    editorChange(editor) {
      console.log('change', editor.children);
      console.log('change', editor.getHtml());
      // editor.focus(true);
    },
    setvalue(val) {
      console.log(val, '+++');
      this.$refs.editor.setHtml(val);
    },
    // 上传图片
    async uploadImageAction(file, insertFn) {
      const imageUrl = await new Promise((resolve, reject) => {
        const formData = new FormData();
        formData.append('file', file);
        formData.append('fileType', 'ordinaryFile');
        this.$axios.post(
          Api.user.uploadFile,
          formData,
          (res) => {
            if (res.isSuccess && res.data) {
              // insertFn(res.data.absolutelyFileUrl, '图片', res.data.absolutelyFileUrl);
              resolve(res.data.absolutelyFileUrl);
            } else {
              this.$message.error('上传失败');
            }
          }
        );
      });
      console.log(imageUrl);
      insertFn(imageUrl, '', imageUrl);
    }
  },
  beforeDestroy() {
    this.logImagePreview('组件 beforeDestroy，移除监听器');
    document.removeEventListener('mousedown', this.previewImage, true);
    clearTimeout(this.imagePreviewTimer);
    const editor = this.editor;
    if (editor == null) return;
    editor.destroy(); // 组件销毁时，及时销毁编辑器
  }
};
</script>

<style lang="scss" scoped>
@import "~@wangeditor/editor/dist/css/style.css";

::v-deep .w-e-bar-item {
  padding: 0;
}

::v-deep .w-e-text-container [data-slate-editor] p {
  margin: 5px 0 !important;
}

::v-deep .w-e-text-placeholder {
  top: 5px;
}

::v-deep .w-e-text-container [data-slate-editor] .w-e-image-container,
::v-deep .w-e-text-container [data-slate-editor] img {
  cursor: zoom-in;
}

.editor {
  width: 100%;
  display: block;

  .rich-editor-ins {
    width: 100%;
    border: 1px solid #ccc;
  }
}
</style>
