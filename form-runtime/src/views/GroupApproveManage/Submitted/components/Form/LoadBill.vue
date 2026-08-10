<!--
 * @Descripttion: 借款单
-->
<template>
  <div class="list-container">
    <h2>借款单</h2>
    <div>
      <el-button type="primary" @click="openPrintDailog">打印</el-button>
    </div>
    <elform :formList="formList" :labelWidth="'180px'" ref="elform"></elform>
    <div class="form-container">
      <h3>费用预算类型</h3>
      <eltables :eltableConfig="costTypeTableConfig" :showAddBt="true" :summaryArr="['money']" showDetail></eltables>
    </div>
    <div class="form-container">
      <h3>入账信息</h3>
      <eltables :eltableConfig="accountInfoTableConfig" :showAddBt="true"></eltables>
    </div>
    <div class="footer-bt">
      <div class="footer-inner">
        <el-button type="primary" plain>保 存</el-button>
        <el-button type="primary">提 交</el-button>
        <el-button plain>取 消</el-button>
      </div>
    </div>
    <printDialog :dialogVisible.sync="printDialogVisible"></printDialog>
  </div>
</template>
<script>
import elform from './components/elCfgForm';
import eltables from './components/eltables';
import printDialog from './components/printDialog';
export default {
  name: 'LoadBill',
  components: { elform, eltables, printDialog },
  data() {
    return {
      formList: [
        [
          {
            label: '借款部门',
            nodeType: 'select',
            prop: 'expenseUnit',
            data: [
              {
                label: '润世华软件和信息',
                value: '1'
              }
            ],
            value: '1',
            isRequire: true
          },
          {
            label: '借款人',
            nodeType: 'input',
            prop: 'applicant',
            disabled: true,
            isRequire: true,
            value: '老刘'
          }
        ],
        [
          {
            label: '项目信息',
            nodeType: 'select',
            prop: 'projInfo',
            data: [
              {
                label: '润世华软件和信息',
                value: '1'
              }
            ],
            value: '1',
            isRequire: true
          },
          {
            label: '借款事由',
            nodeType: 'text',
            prop: 'travelReason',
            isRequire: true
          }
        ],
        [
          {
            label: '付款单位',
            nodeType: 'select',
            prop: 'projInfo',
            data: [
              {
                label: '润世华软件和信息',
                value: '1'
              }
            ],
            value: '1',
            isRequire: true
          },
          {
            label: '付款方式',
            nodeType: 'select',
            prop: 'projInfo',
            data: [
              {
                label: '润世华软件和信息',
                value: '1'
              }
            ],
            value: '1',
            isRequire: true
          }
        ],
        [
          {
            label: '申请借款金额（元）',
            nodeType: 'input',
            prop: 'projInfo'
          },
          {
            label: '大写',
            nodeType: 'input',
            prop: 'travelReason',
            disabled: true
          }
        ],
        [
          {
            label: '前期借款未核销金额（元）',
            nodeType: 'input',
            prop: 'projInfo'
          },
          {
            label: '大写',
            nodeType: 'input',
            prop: 'travelReason',
            disabled: true
          }
        ],
        [
          {
            label: '还款计划',
            nodeType: 'text',
            prop: 'projInfo'
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
      costTypeTableConfig: [
        {
          label: '序号',
          isRequire: true,
          prop: 'number',
          slot: {
            nodeType: 'select',
            source: [
              { label: '住宿费', value: '1' },
              { label: '餐费', value: '2' }
            ]
          }
        },
        {
          label: '费用预算类型',
          isRequire: true,
          prop: 'type',
          slot: {
            nodeType: 'cascader',
            source: [
              {
                value: 'shejiyuanze',
                label: '设计原则',
                children: [{
                  value: 'yizhi',
                  label: '一致'
                }, {
                  value: 'fankui',
                  label: '反馈'
                }, {
                  value: 'xiaolv',
                  label: '效率'
                }, {
                  value: 'kekong',
                  label: '可控'
                }]
              }, {
                value: 'daohang',
                label: '导航',
                children: [{
                  value: 'cexiangdaohang',
                  label: '侧向导航'
                }, {
                  value: 'dingbudaohang',
                  label: '顶部导航'
                }]
              }
            ]
          }
        },
        {
          label: '金额（元）',
          isRequire: true,
          prop: 'money',
          slot: {
            nodeType: 'moneyInput'
          }
        },
        {
          label: '操作',
          isRequire: false,
          width: '80',
          prop: '',
          slot: {
            nodeType: 'icon',
            icon: 'el-icon-delete'
          }
        }
      ],
      accountInfoTableConfig: [
        {
          label: '单位或姓名',
          isRequire: true,
          prop: 'name',
          slot: {
            nodeType: 'input'
          }
        },
        {
          label: '开户行',
          isRequire: true,
          prop: 'name',
          slot: {
            nodeType: 'input'
          }
        },
        {
          label: '账号',
          isRequire: true,
          prop: 'name',
          slot: {
            nodeType: 'input'
          }
        },
        {
          label: '金额（元）',
          isRequire: true,
          prop: 'name',
          slot: {
            nodeType: 'moneyInput'
          }
        }, {
          label: '操作',
          isRequire: false,
          width: '80',
          prop: '',
          slot: {
            nodeType: 'icon',
            icon: 'el-icon-delete'
          }
        }
      ],
      printDialogVisible: false
    };
  },
  methods: {
    openPrintDailog() {
      this.printDialogVisible = true;
    }
  }
};

</script>
<style scoped lang="scss">
h2 {
  text-align: center;
}

h3 {
  line-height: 30px;
}

.list-container {
  width: 88%;
  margin: 10px auto 0;
}

.form-container {
  margin-top: 10px;
  padding: 25px;
  box-sizing: border-box;
  border: 1px solid #e2e2e2;
  border-radius: 4px;

}

.footer-bt {
  text-align: center;
  background: transparent;
  width: 100%;
  position: absolute;
  left: 0;
  bottom: 10px;
  z-index: 1000;
  padding: 15px 10px;

  .footer-inner {

    background: #fff;
  }
}
</style>
