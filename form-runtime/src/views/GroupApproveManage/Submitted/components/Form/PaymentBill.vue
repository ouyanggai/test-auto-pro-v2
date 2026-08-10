<!--
 * @Descripttion: 请款单
-->
<template>
  <div class="list-container">
    <h2>请款单</h2>
    <elform :formList="formList" :labelWidth="'180px'" ref="elform"></elform>
    <div class="form-container">
      <h3>费用预算类型</h3>
      <eltables :eltableConfig="costTypeTableConfig" :showAddBt="true" :summaryArr="['money']"></eltables>
    </div>
    <div class="footer-bt">
      <div class="footer-inner">
        <el-button type="primary" plain>保 存</el-button>
        <el-button type="primary">提 交</el-button>
        <el-button plain>取 消</el-button>
      </div>
    </div>
  </div>
</template>
<script>
import elform from './components/elCfgForm';
import eltables from './components/eltables';
import billTable from './components/billTable';
export default {
  name: 'PaymentBill',
  components: { elform, eltables, billTable },
  data() {
    return {
      formList: [
        [
          {
            label: '请款部门',
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
            label: '经手人',
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
            label: '请款事由',
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
            label: '请款金额（元）',
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
            label: '收款人',
            nodeType: 'input',
            prop: 'projInfo'
          }
        ],
        [
          {
            label: '收款人开户行',
            nodeType: 'input',
            prop: 'projInfo'
          },
          {
            label: '收款人账号',
            nodeType: 'input',
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
            nodeType: 'input'
          }
        },
        {
          label: '操作',
          isRequire: false,
          prop: '',
          slot: {
            nodeType: 'icon',
            icon: 'el-icon-delete'
          }
        }
      ]
    };
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
