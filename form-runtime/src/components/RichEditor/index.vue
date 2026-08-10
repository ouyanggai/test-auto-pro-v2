<!--
 * @Author: junshao
 * @Date: 2022-12-07 10:15:03
 * @LastEditors: hejing
 * @LastEditTime: 2025-05-30 10:01:04
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
/**
 * 工具栏实例：DomEditor.getToolbar(editor)
 * 需要排除的工具：toolbar.excludeKeys
 * 获取编辑器所有菜单: editor.getAllMenuKeys()
 * 菜单默认配置：editor.getMenuConfig(key)
 */
export default {
  components: { Editor, Toolbar },
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
    }
  },
  mounted() {
  },
  methods: {
    // 编辑器创建后的回调
    onCreated(editor) {
      // 获取工具栏所有工具
      // console.log(editor.getAllMenuKeys());
      this.editor = Object.seal(editor); // 一定要用 Object.seal() ，否则会报错
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

.editor {
  width: 100%;
  display: block;

  .rich-editor-ins {
    width: 100%;
    border: 1px solid #ccc;
  }
}
</style>