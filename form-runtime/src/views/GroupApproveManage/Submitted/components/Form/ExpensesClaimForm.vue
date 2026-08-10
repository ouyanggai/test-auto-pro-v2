<!--
 * @Descripttion: 差旅报销单
 * @Author: liufuze
-->
<template>
  <div class="list-container">
    <h2>费用报销单</h2>
    <elform :formList="formList" ref="elform"></elform>
    <div class="form-container">
      <h3>费用明细</h3>
      <eltables :eltableConfig="costTableConfig" :showAddBt="true"></eltables>
    </div>
    <div class="form-container">
      <h3>费用预算类型</h3>
      <eltables :eltableConfig="costTypeTableConfig" :showAddBt="true" showDetail></eltables>
    </div>
    <div class="form-container">
      <h3>发票信息</h3>
      <billTable></billTable>
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
import elform from './components/elCfgForm';
import eltables from './components/eltables';
import billTable from './components/billTable';
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
import { deepClone } from '@/utils';
export default {
  name: 'TravelExpenseForm',
  components: { elform, eltables, billTable },
  data() {
    return {
      rules: {
        date: [{ required: true, message: '请输入', trigger: 'blur' }]
      },
      allPerson: [],
      formList: [
        [
          {
            label: '报销单位',
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
            label: '申请人',
            nodeType: 'input',
            prop: 'applicant',
            disabled: true,
            isRequire: true,
            value: '老刘'
          }
        ], [
          {
            label: '项目信息',
            nodeType: 'input',
            prop: 'projInfo'
          },
          {
            label: '单据附件数量（张）',
            nodeType: 'number',
            min: 0,
            prop: 'nums',
            isRequire: true
          }
        ],
        [
          {
            label: '备注',
            nodeType: 'text',
            prop: 'remark'
          }, {
            label: '是否冲借（请）款',
            nodeType: 'radio',
            data: [
              {
                label: 1,
                value: '冲借款'
              },
              {
                label: 2,
                value: '冲请款'
              }
            ],
            prop: 'type'
          }
        ], [
          {
            nodeType: 'upload',
            modelName: 'expenseUnit',
            prop: 'file'
          }
        ]
      ],
      costTableConfig: [
        {
          label: '费用类型',
          isRequire: true,
          prop: 'type',
          slot: {
            nodeType: 'select',
            source: [
              { label: '住宿费', value: '1' },
              { label: '餐费', value: '2' }
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
          label: '备注',
          isRequire: false,
          prop: 'remark',
          slot: {
            nodeType: 'input'
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
          width: '80',
          prop: '',
          slot: {
            nodeType: 'icon',
            icon: 'el-icon-delete'
          }
        }
      ]
    };
  },
  created() {

  },
  computed: {
  },

  methods: {

    cancel() {

    },
    submit() { }
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

::v-deep .table-header {
  height: 40px;
  font-size: 14px;
  // th {
  //   background: gray;
  // }
}

::v-deep .el-date-editor.el-input,
::v-deep .el-date-editor.el-input__inner {
  width: 120px !important;
}

.form-container {
  margin-top: 10px;
  padding: 25px;
  box-sizing: border-box;
  border: 1px solid #e2e2e2;
  border-radius: 4px;

}

.travel .el-form-item.el-form-item--mini {
  margin-bottom: 0;
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
