<template>
  <el-dialog :visible="visible"
    v-loading="loading"
   width="90%" 
    top="20px" :close-on-click-modal="false" :append-to-body="true" class="adjust-department-dialog"
    @close='handleClose'>
    <div class="approval-form-dialog-body">
      <div class="approval-form-dialog-form">
        <fm-generate-form :data="jsonData" :value="editData"  ref="generateFormA" >
        </fm-generate-form>
      </div>
      <div class="approval-form-dialog-log">
        <FlowLog :flowInstanceId="flowInstanceId" :logTableData="logTableData" :isNoEnterprise="false"></FlowLog>
      </div>
    </div>

    <!-- <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
    </span> -->
  </el-dialog>
</template>

<script>
import Api from '@/api';
import FlowLog from '../../components/flowLog.vue';
import mixin from '../../components/mixin';
export default {
  name: 'ApprovalFormDialog',
  mixins: [mixin],
  components: {
    FlowLog
  },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    flowInstanceId:{
      type: String,
      default: ''
    },
    formId: {
      type: String,
      default: ''
    },
  },
  data(){
    return{
      jsonData:{},
      editData:{},
      disabledData:[],
      enableData: [],
      models:{},
      logTableData: [],
      loading:false
    }
  },
  created(){
    this.getFormDetail();
    this.flowInstanceId && this.fetchLogData();
  },
  methods:{
    handleClose() {
      this.$emit('update:visible', false);
    },
    // 获取表单模板
    async getFormDetail() {
      this.loading = true;
      console.log('=========getFormDetail2=========')
        this.disabledData = [];
        this.jsonData = {
          config: {},
          list: []
        };

      const editData = this.models = await this.getFormData();
      for (const item in editData) {
        this.$set(this.editData, item, editData[item]);
      }
      const enableData = []
      this.$axios.post(
        Api.qualityManage.getTaskFormDetail,
        {
          data: {
            id: this.formId
          }
        },
        (res) => {
          this.loading = false;
          if (res.isSuccess) {
            // this.jsonData = JSON.parse(res.data.templateData);
            let copyTemplateData = JSON.parse(res.data.templateData);
            this.setRequireByPermission(copyTemplateData.list);
            this.jsonData = JSON.parse(JSON.stringify(copyTemplateData));

            const fieldsTemplateList = res.data.fieldsTemplateList;
            const disabledData = fieldsTemplateList.map(item => {
              return item.englishName;
            });
            this.disabledData = disabledData;
            this.$nextTick(() => {
              this.$refs.generateFormA.refresh();
              this.$refs.generateFormA.disabled(disabledData, true);
              if (this.btnVisible) {
                this.$refs.generateFormA.disabled(enableData, false);
              }
            });
          }
        }
      );
    },
    // 获取表单字段值
    getFormData() {
      return new Promise((resolve, reject) => {
        this.$axios.post(
          Api.qualityManage.getFormData,
          {
            data: {
              id: this.flowInstanceId
            },
            // excludeFieldNames: ['auto_audit_info_']
          },
          (res) => {
            if (res.isSuccess) {
              resolve(res.data.data);
            }
          }
        );
      });
    },
    // 递归表单，根据配置的表单权限，修改表单是否校验配置（如果某个表单字段本身设置必填，但是没有配置权限，那么要改为非必填；如果本身未设置必填，就算有权限，也不需要必填）
    setRequireByPermission(genList) {
      genList.map((item, key) => {
        if (item.type == 'grid') {
          item.columns.map(val => {
            this.setRequireByPermission(val.list);
          });
        } else if (item.type == 'report') {
          item.rows.map(val => {
            val.columns.map(i => {
              this.setRequireByPermission(i.list);
            });
          });
        } else if (item.type == 'inline') {
          this.setRequireByPermission(item.list);
        } else {
          if (item.model) {
            if (!this.enableData.includes(item.model)) {
              // 下面两行代码足够覆盖所有场景，即用下面两行；不够用可把注释打开，根据不同场景进行配置。
              // if (item.options.validatorCheck) { // 自定义校验规则
              //   this.$set(item.options,'validatorCheck',false);
              // } else if (item.options.validatorCheck && item.options.required) { // 自定义校验规则还点了必填
              //   this.$set(item.options,'validatorCheck',false);
              // } else if (item.options.patternCheck && item.options.required) { // 正则校验
              //   this.$set(item.options,'patternCheck',false);
              // } else if (item.options.dataTypeCheck && item.options.required) { // 表单内置的校验规则
              //   this.$set(item.options,'dataTypeCheck',false);
              // }
              this.$set(item.options,'required',false);
              this.$set(item,'rules',[]);
              // console.log('item2',item)
            }
          }
        }
      });
    },
  }
}

</script>

<style lang="scss" scoped>
.approval-form-dialog-body {
  display: flex;
  gap: 16px;
  height: calc(100vh - 110px);
  overflow: hidden;
}

.approval-form-dialog-form {
  flex: 1;
  min-width: 0;
  overflow: auto;
}

.approval-form-dialog-log {
  width: 360px;
  flex-shrink: 0;
  overflow: auto;
}

</style>
