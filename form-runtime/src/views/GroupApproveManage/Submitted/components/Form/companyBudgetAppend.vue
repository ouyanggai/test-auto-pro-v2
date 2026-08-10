<!--
 * @Descripttion: 年度预算追加
 * @Author: liufuze
-->
<template>
  <div class="outer">
    <div class="container">
      <h2>追加公司年度预算</h2>
      <div class="inner-container">
        <el-card class="box-card">
          <h3 style="margin-bottom:30px">追加公司年度预算</h3>
          <elform ref="elform" :formList="formList"></elform>
        </el-card>
        <cardTable :infoData="datas" :cardConfig="config" ref="cardTableInfo"></cardTable>
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
import { localstorageGet } from '@/utils/auth';
import eleupload from '@/components/EleUpload';
import Api from '@/api';
import { deepClone } from '@/utils';
import mixin from './mixins/mixins';
import cardTable from './components/cardTable'
import elform from './components/elCfgForm'
export default {
  name: 'CompanyBudgetAppend',
  mixins: [mixin],
  components: { eleupload, cardTable, elform },
  data() {
    return {
      disableCompany: false,
      dateOption: [],
      formList: [
        [
          {
            label: '公司名称',
            nodeType: 'select',
            prop: 'companyId',
            isRequire: true,
            span: 8,
            data: [],
            value: '',
            isRequire: true,
          },
          {
            label: '预算年度',
            nodeType: 'date-picker',
            type: 'year',
            prop: 'annual',
            isRequire: true,
            span: 8,
            value: '',
          },
          {
            label: '追加预算金额(万)',
            nodeType: 'number',
            prop: 'money',
            disabled: true,
            isRequire: true,
            span: 8,
            value: '10',
            controls: false
          }
        ], [
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
      datas: [
      ],
      companyOption: [],
      config: {
        cardTitle: '预算详情',
        addButton: true,
        // selectTempButton: true,
        previewButton: true,
        budgetTemp: {
          budgetType: '',
          isRelateProj: false,
          relateProjName: '',
          relateProjId: '',
          appendMoney: '0.00'
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
          relateProj: {
            isShow: true,
            isShowRadio: true,
          },
          appendMoney: {
            isShow: true,
            prop: 'appendMoney',
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
      },
    };
  },
  inject: ['prevStepHandle', 'sumbitFlow'],
  created() {
    this.getMainDuty()
  },
  methods: {
    /**
     *
     */
    submit(status) {
      this.$refs.elform.getData().then(formDataRes => {
        var formData = formDataRes
        this.$refs.cardTableInfo.validData().then(tableDataRes => {
          var cardTableData = tableDataRes
          //     //TODO先提交业务，再提交流程，需要把业务id拿到，传入
          //     console.log('这里需要调用提交业务的接口')
          //     //this.sumbitFlow('业务id')
        })
      })
    },
  }
};
</script>
<style lang="scss" scoped src="./style/style.scss"></style>
<style scoped>
::v-deep .flow-content {
  scroll-behavior: smooth;
}

.line {
  width: 100%;
  height: 1px;
  background: rgb(235, 235, 235);
  margin: 3px 0;
}
</style>
