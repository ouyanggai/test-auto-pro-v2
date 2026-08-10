<!--
 * @Descripttion: 公司月度预算上报
 * @Author: liufuze
-->
<template>
  <div class="outer">
    <div class="container">
      <h2>公司月度预算</h2>
      <div class="inner-container">
        <el-card class="box-card">
          <h3 style="margin-bottom:30px">预算基本信息</h3>
          <elform ref="elform" :formList="formList"></elform>
        </el-card>
        <cardTable :infoData="datas" :cardConfig="config" ref="cardTableInfo" @validComplete="validComplete"></cardTable>
      </div>
    </div>
    <div class="footer-bt">
      <div class="footer-inner">
        <el-button type="primary" plain @click="submit(0)">保 存</el-button>
        <el-button type="primary" @click="submit(1)">提 交</el-button>
        <el-button @click="cancel" plain>取 消</el-button>
      </div>
    </div>
  </div>
</template>

<script>
import { localstorageSet, localstorageRemove, localstorageGet } from '@/utils/auth';
import Api from '@/api';
import { deepClone } from '@/utils';
import cardTable from './components/cardTable';
import elform from './components/elCfgForm';
import mixin from './mixins/mixins';
export default {
  name: 'CompanyMouthlyBudget',
  components: { cardTable, elform },
  mixins: [mixin],
  data() {
    return {
      form: {
        companyId: '', // localstorageGet('companyId'),
        projectId: '',
        month: '',
        money: '',
        remarks: '',
        departName: ''
      },
      datas: [
        // {
        //   departName: '',
        //   departId: '',
        //   activeNames: '1',
        //   budget: [
        //     {
        //       budgetType: ''
        //     }
        //   ]
        // }
      ],
      originDepartData: [],
      departOptions: [],
      projectOptions: [],
      hasInfo: false, // 选择月度和部门后是否有预算信息
      selectFlowType: 'depart_monthly_budget',
      type: 'init',
      attachFile: [],

      yearBudget: {},
      enableData: [],
      formList: [
        [
          {
            label: '公司名称',
            nodeType: 'select',
            prop: 'companyId',
            isRequire: true,
            span: 12,
            data: [],
            value: '',
            isRequire: true
          },
          {
            label: '月份',
            nodeType: 'date-picker',
            type: 'month',
            prop: 'month',
            isRequire: true,
            span: 12,
            value: '',
            pickerOptions: {
              disabledDate(date) {
                // disabledDate 文档上：设置禁用状态，参数为当前日期，要求返回 Boolean
                const currentYear = new Date().getFullYear();
                return (
                  date.getTime() < new Date(currentYear, 0, 1).getTime() ||
                  date.getTime() > new Date(currentYear, 11, 31).getTime()
                );
              }
            }
          }
        ],
        [
          {
            label: '下月预算金额(万)',
            nodeType: 'number',
            prop: 'money',
            disabled: true,
            isRequire: true,
            span: 12,
            value: '10',
            controls: false
          }
        ],
        [
          {
            label: '预算金额分析',
            nodeType: 'text',
            prop: 'remark',
            isRequire: true,
            span: 16,
            value: '',
            limit: 5000,
            minRows: 4,
            maxRows: 8
          }
        ], [
          {
            nodeType: 'upload'
          }
        ]
      ],

      config: {
        cardTitle: '预算详情',
        addButton: true,
        departOptions: [],
        projectOptions: [],
        budgetTemp: {
          budgetType: '',
          isRelateProj: false,
          relateProjName: '',
          relateProjId: '',
          budgetMoney: '0.00'
        },
        departBudgetTemp: {
          departName: '',
          departId: '',
          activeNames: '1',
          budget: [
          ]
        },
        tableInfo: {
          budgetType: {
            disabled: true
          },
          relateProj: {
            isShow: true,
            isShowRadio: false,
            width: '150px',
            disabled: true
          },
          budgetMoney: {
            isShow: true,
            prop: 'budgetMoney',
            width: '160px'
          },
          cause: {
            isShow: true
          },
          explain: {
            isShow: true
          },
          remark: {
            isShow: true
          }
        }
      }

    };
  },
  inject: ['prevStepHandle', 'sumbitFlow'],
  computed: {
  },
  created() {
    this.getMainDuty();
  },
  mounted() {
  },
  methods: {
    submit(status) {
      // this.$refs.cardTableInfo.validData()
    },
    validComplete(data) {
      if (data) {
        // TODO先提交业务，再提交流程，需要把业务id拿到，传入
        console.log('这里需要调用提交业务的接口');
        // this.sumbitFlow('业务id')
      } else {
        console.log('数据校验不通过');
      }
    },

    cancel() {
      this.$confirm('确认取消?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.prevStepHandle();
      });
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
    },
    clearAllFile() {
      this.$axios.post(
        Api.schedule.deleteAttachment,
        {
          ids: [this.bizId]
        }
      );
    }
  }
};
</script>
<style lang="scss" scoped src="@/views/GroupApproveManage/Submitted/components/Form/style/style.scss"></style>
