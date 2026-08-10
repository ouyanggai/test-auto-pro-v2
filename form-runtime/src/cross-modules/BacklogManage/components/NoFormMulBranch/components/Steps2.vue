<!--
 * @Descripttion:创建新流程\表单设计
 * @Author: Calvin
 * @Date: 2021-06-04 11:04:14
-->
<template>
  <div class="container">
    <el-form
      ref="form"
      :model="form"
      label-width="100px"
      label-position="left"
    >
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item
            label="表单名称"
            prop="name"
          >
            <el-input
              v-model="form.name"
              style="width: 400px;"
              :disabled="editType ==3"
              placeholder="请输入表单名称"
            />
          </el-form-item>
        </el-col>
      </el-row>

    </el-form>
    <div class="designer-container">
      <fm-making-form
        v-if="editType != 3"
        ref="makingform"
        style="height: 100%;"
        preview
        upload
        clearable
        generate-code
        generate-json
      />
      <div
        v-else
        style="width:100%;height:100%"
      >

        <fm-generate-form
          ref="generateForm"
          :data="form.jsonData"
          :edit-data="editData"
        />
      </div>

    </div>

  </div>
</template>

<script>

export default {
  name: '',
  components: {
  },
  props: {
    editType: {
      type: [String, Number],
      default: 1
    },
    form: {
      type: Object,
      default: () => {
        return {
          name: '',
          jsonData: {

          }
        };
      }
    }
  },
  data() {
    return {
      generateFormVisible: false,
      editData: {}
    };
  },
  computed: {},
  watch: {
    'form.jsonData'(val) {
      // if (val) {
      //   if (this.editType != 1) {
      //     this.$nextTick(() => {
      //       this.$refs.generateForm.refresh();
      //     });
      //   }
      // }
      if (val) {
        if (this.editType != 3) {
          this.$refs.makingform.setJSON(this.form.jsonData);
        } else {
          this.$nextTick(() => {
            this.$refs.generateForm.refresh();
            this.$refs.generateForm.getData().then(value => {
              const disabledData = Object.keys(value);
              this.$refs.generateForm.disabled(disabledData, true);
            });
          });
        }
      }
    }
  },
  created() { },
  mounted() { },
  methods: {

  }
};
</script>

<style scoped lang="scss">
.container {
  width: 100%;
  height: 100%;
  .design {
    text-align: left;
  }
  .designer-container {
    height: calc(100% - 50px);
  }
}
</style>
<style lang="scss">
// @import "@/assets/styles/formMaking.scss";
</style>
