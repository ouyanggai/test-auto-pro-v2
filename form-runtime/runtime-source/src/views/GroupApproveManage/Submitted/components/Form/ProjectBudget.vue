<!--  -->
<template>
  <div class="outer">
    <div class="container">
      <h2>项目预算立项</h2>
      <div class="inner-container">
        <el-card class="box-card">
          <h3 style="margin-bottom:30px">预算基本信息</h3>
          <elform :formList="formList" ref="elform"></elform>
        </el-card>
        <cardTable :infoData="datas" :cardConfig="config" ref="cardTableInfo" @validComplete="validComplete"></cardTable>
      </div>
      <div class="footer-bt">
        <div class="footer-inner">
          <el-button type="primary" plain @click="submit(0)">保 存</el-button>
          <el-button type="primary" @click="submit(1)">提 交</el-button>
          <el-button @click="cancel" plain>取 消</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { localstorageSet, localstorageRemove, localstorageGet } from '@/utils/auth';
import Api from '@/api';
import { deepClone } from '@/utils/index';
import cardTable from './components/cardTable';
import elform from './components/elCfgForm';
import mixin from './mixins/mixins';
export default {
  name: 'ProjectBudget',
  components: { cardTable, elform },
  mixins: [mixin],
  data() {
    var userName = this.$store.state.user.userName;
    return {
      attachFile: [],
      formList: [
        [
          {
            label: '立项部门',
            nodeType: 'select',
            prop: 'expenseUnit',
            isRequire: true,
            data: [],
            value: '',
            isRequire: true
          },
          {
            label: '申请人',
            nodeType: 'input',
            prop: 'applicant',
            disabled: true,
            isRequire: true,
            value: userName
          }
        ], [
          {
            label: '项目名称',
            nodeType: 'select',
            prop: 'projInfo',
            isRequire: true,
            data: []
          },
          {
            label: '立项编号',
            nodeType: 'input',
            prop: 'travelReason',
            isRequire: false
          }
        ],
        [
          {
            label: '项目确立时间',
            nodeType: 'date-picker',
            type: 'date',
            prop: 'isOverQuota',
            isRequire: true
          },
          {
            label: '项目标的金额(万)',
            nodeType: 'number',
            min: 0,
            prop: 'nums',
            isRequire: true,
            controls: false
          }
        ],
        [
          {
            label: '项目预算金额(万)',
            nodeType: 'number',
            min: 0,
            prop: 'nums',
            isRequire: true,
            controls: false
          },
          {
            label: '项目情况',
            prop: 'projType',
            isRequire: true,
            nodeType: 'radio',
            data: [
              {
                label: 1,
                value: '前期'
              },
              {
                label: 2,
                value: '过程'
              },
              {
                label: 3,
                value: '结束'
              }
            ]
          }
        ],
        [
          {
            label: '归属',
            nodeType: 'input',
            prop: 'belong'
          }
        ],
        [
          {
            label: '预算金额分析',
            nodeType: 'text',
            prop: 'remark',
            span: 20,
            limit: 5000,
            minRows: 4,
            maxRow: 6,
            isRequire: true
          }
        ],
        [
          {
            nodeType: 'upload',
            modelName: 'expenseUnit',
            prop: 'file'
          }
        ]
      ],
      datas: [],
      config: {
        cardTitle: '当年费用预算计划',
        addButton: true,
        budgetTemp: {
          budgetType: '',
          isRelateProj: false,
          relateProjName: '',
          relateProjId: '',
          budgetMoney: 0
        },
        departBudgetTemp: {
          departName: '',
          departId: '',
          activeNames: '1',
          budget: [
          ]
        },
        departOptions: [],
        projectOptions: [],
        tableInfo: {
          budgetType: {
            isSelect: true
          },
          useBudget: {
            isShow: true,
            width: '140px'
          },
          projectBudgetMoney: {
            isShow: true,
            prop: 'projectBudgetMoney',
            width: '160px'
          },
          appendToBudget: {
            isShow: true,
            prop: 'appendToBudget',
            width: '160px'
          },
          operation: {
            isShow: true,
            width: '80px'
          },
          addLineButton: {
            isShow: true
          }
        }
      }
    };
  },
  created() {
    this.getMainDuty();
  },
  watch: {
    'config.projectOptions'(val) {
      this.formList[1][0].data = val;
    }
  },
  methods: {
    validComplete() {

    },
    submit() {
      console.log(this.$refs.elform.getData());
    },
    async getProjectBudgetById() {
      const query = {
        id: this.id
      };
      return await this.$axios.post(
        Api.annualBudget.findBudgetById,
        {
          data: query
        }
      );
    },
    async changeProject(projectId) {
      const index = this.projectOptions.findIndex(el => el.id == projectId);
      if (index > -1) {
        const companyId = this.projectOptions[index].companyId;
        await this.getProjectBudget();
        await this.getDepartByCompanyId(companyId);
        if (this.form.companyId != '') {
          if (this.form.companyId != companyId) {
            // 清空所有部门
            this.form.companyId = companyId;
            this.$message.info('所选项目所在公司和原项目所在公司不同，部门信息将被清除');
            for (const k in this.datas) {
              this.datas[k].forEach(item => {
                item.departName = '';
                item.departmentId = '';
              });
            }
          }
        } else {
          this.form.companyId = companyId;
        }
        await this.getFlowType();
      } else {

      }
    },
    findBudgetById(datas, budgetTypeId) {
      let x; let y; let has = false;
      for (let i = 0; datas[i]; i++) {
        const budget = datas[i].budget;
        const index = budget.findIndex(item => item.budgetTypeId == budgetTypeId);
        if (index > -1) {
          x = i, y = index, has = true;
          break;
        }
      }
      return { has, x, y };
    },
    cancel() {
      if (this.type == 'detail') {
        this.$router.go(-1);
      } else {
        this.$confirm('确认取消?', '提示', {
          closeOnClickModal: false,
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.$router.go(-1);
        });
      }
    },
    clearAllFile() {
      this.$axios.post(
        Api.schedule.deleteAttachment,
        {
          ids: [this.bizId]
        }
      );
    },
    bindFileById(relationId, fileId) {
      const data = {
        relationId,
        fileId
      };
      return this.$axios.post(
        Api.schedule.saveAttachment,
        { data }
      );
    },
    // 根据业务id获取文件
    getFileByBizId(id) {
      this.$axios.post(
        Api.schedule.getAttachmentList, {
          data: {
            relationId: id
          }
        }).then(res => {
        if (res.isSuccess) {
          const list = res.data;
          const attachFile = list.map(item => {
            return {
              id: item.id,
              fileName: item.fileName,
              fileUrl: item.fileUrl
            };
          });
          this.attachFile = attachFile;
        }
      });
    }

  }
};
</script>
<style lang="scss" scoped src="@/views/GroupApproveManage/Submitted/components/Form/style/style.scss"></style>
<style scoped>
table {
  border-collapse: separate;
  border-spacing: 0;
}

::v-deep .el-input-number--mini {
  width: 100%;
}
</style>
