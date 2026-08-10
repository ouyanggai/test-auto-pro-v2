<template>
  <div>
    <el-dialog :visible="dialogVisible" width="680px" :before-close="handleClose" append-to-body center
      :close-on-click-modal="false">
      <div style="width:590px;">
        <div ref="print" class="print">
          <h2>{{ printData.type }}</h2>
          <div class="print-container" style="margin-top:15px">
            <!-- 基本信息 -->
            <printForm :formList="printData.basicInfo" :labelWidth="'160px'" ref="elform"></printForm>
          </div>
          <div class="print-container">
            <h3>费用预算类型</h3>
            <printTable :printTableConfig="printData.costTypeTableConfig" :printTableData="printTableData"></printTable>
          </div>
        </div>
      </div>
      <span slot="footer" class="dialog-footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="print">打 印</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import printForm from './printForm';
import printTable from './printTable';
export default {
  components: { printForm, printTable },
  props: {
    dialogVisible: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      labelWidth: '140px',
      printData: {
        type: '借款单',
        basicInfo: [
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
              isRequire: true,
              value: '22222222'
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
          }
        ]
      },
      printTableData: [
        { number: '1' }, { number: '1' }, { number: '1' }, { number: '1' }, { number: '1' },
        { number: '1' }, { number: '1' }, { number: '1' }, { number: '1' }, { number: '1' }, { number: '1' }, { number: '1' }, { number: '1' }, { number: '1' }, { number: '1' },
        { number: '1' }, { number: '1' }, { number: '1' }
      ]
    };
  },
  methods: {
    print() {
      this.$print(this.$refs.print);
      // this.dialogVisible = false;
    },
    handleClose() {
      this.$emit('update:dialogVisible', false);
    },
    getExpendDetailSummaries(param) { // 费用明细表格合计
      // if (this.summaryArr.length == 0) return;
      const { columns, data } = param;
      const sums = [];
      columns.forEach((column, index) => {
        if (this.summaryArr.includes(column.property)) {
          const values = data.map(item => Number(item[column.property]));
          if (!values.every(value => isNaN(value))) {
            sums[index] = values.reduce((prev, curr) => {
              const value = Number(curr);
              if (!isNaN(value)) {
                return prev + curr;
              } else {
                return prev;
              }
            }, 0);
            sums[index] = '合计：' + sums[index].toFixed(2) + ' 元';
          } else {
            sums[index] = '';
          }
        }
      });
      return sums;
    }
  }
};
</script>

<style lang="scss" scoped>
@media print {
  @page {
    size: auto; //打印可以选择布局：横向，纵向
    // size: landscape;//横向
    // size: portrait;//纵向
    // margin: 23.5mm; //默认边距
  }
}

.print {
  h2 {
    text-align: center;
  }

  .print-container {
    margin-top: 10px;
    border: 1px solid #e2e2e2;
  }

  .dialog-footer {
    text-align: center;
  }
}
</style>
