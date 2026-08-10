<!--
 * @description: 指标分解弹窗
 * @Author: zhengzetao
 * @Date: 2024.3.11
-->
<template>
  <el-dialog :visible="visible" title="指标分解" :close-on-click-modal="false" width="60%" top="100px" @close='handleClose'
    class="examiner-dialog" append-to-body>
    <div style="display:flex;">
      <div style="margin-right: 20px;margin-bottom: 24px;">指标分解</div>
      <div>
        <el-radio-group v-model="radio">
          <el-radio :label="'month'" border style="margin-right: 10px;">按月分解</el-radio>
          <el-radio :label="'quarterly'" border>按季度分解</el-radio>
        </el-radio-group>
      </div>
    </div>
    <el-alert title="分解合同额、开票、回款、净利润时，“完成标准”带上“合同额”、“开票”、“回款”、“净利润”文字，系统会自动识别其类型和金额等内容，例如:“完成合同额10万元”、“完成合同开票50000”、“完成净利润3亿元”"
    type="success" :closable="false" style="margin-bottom: 10px;">
    </el-alert>
    <div v-if="radio == 'month'">
      <el-table
        :data="monthIndexData"
        style="width: 100%">
        <el-table-column
          label="月度"
          width="100">
          <template slot-scope="scope">
            <div class="style-common month-style">{{scope.row.targetTime}}月</div>
          </template>
        </el-table-column>
        <el-table-column
          label="完成标准">
          <template slot-scope="scope">
            <div v-if="isHalfOrYearType" v-html="scope.row.content"></div>
            <el-input v-else v-model="scope.row.content" placeholder="请输入内容"></el-input>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div v-if="radio == 'quarterly'">
      <el-table
        :data="quarterIndexData"
        style="width: 100%">
        <el-table-column
          label="季度"
          width="100">
          <template slot-scope="scope">
            <div class="style-common quarter-style">第{{scope.row.targetTime}}季度</div>
          </template>
        </el-table-column>
        <el-table-column
          label="完成标准">
          <template slot-scope="scope">
            <div v-if="isHalfOrYearType" v-html="scope.row.content"></div>
            <el-input v-else v-model="scope.row.content" placeholder="请输入内容"></el-input>
          </template>
        </el-table-column>
        <el-table-column
          label="完成占比（%）" width="200">
          <template slot-scope="scope">
            <el-input placeholder="请输入内容" v-model="item.weight" v-for="item in scope.row.kpiSplitItemWeights" style="margin-bottom:4px;" :disabled="isHalfOrYearType">
              <template slot="prepend">{{ item.targetTime }}月</template>
              <template slot="append">%</template>
            </el-input>
          </template>
        </el-table-column>
      </el-table>
    </div>
    

    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button type="primary" @click="submit">确 定</el-button>
    </span>
  </el-dialog>
</template>

<script>
import Api from '@/api';
export default {
  name: '',
  components: {},
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    isHalfOrYearType: {
      type: Boolean,
      default: false
    },
    kpiSplitItems:{
      type:Array,
      default:function(){
        return []
      }
    }
  },
  watch: {
  },
  data() {
    return {
      radio: 'month',
      // let data = {
          //     keyPerformanceIndicatorsList:[{
          //         kpiSplitItems:[{
          //           targetTime:3,//分解项的目标时间，整型，季度1-4，月度1-12
          //           content:"这里是考核标准",
          //           kpiSplitType:"quarterly",//  quarterly为季度，month为月度，季度kpiSplitItemWeights不能为null
          //           kpiSplitItemWeights:[{//月度占比
          //             targetTime:8,//月底占比目标时间，1季度1-3，2季度4-6，3季度7-9，4季度10-12
          //             weight:0.4//占比，double类型，0.4为40%
          //           }]
          //       }]
          //     }]
          // }

      monthIndexData:[
        {
          targetTime:'1',
          content:'',
          kpiSplitType:"month",
        },
        {
          targetTime:'2',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'3',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'4',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'5',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'6',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'7',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'8',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'9',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'10',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'11',
          content:'',
          kpiSplitType:"month"
        },
        {
          targetTime:'12',
          content:'',
          kpiSplitType:"month"
        },
      ],
      quarterIndexData:[
        {
          targetTime:'1',
          kpiSplitItemWeights:[
            {
              targetTime:'1',
              weight:'0'
            },
            {
              targetTime:'2',
              weight:'0'
            },
            {
              targetTime:'3',
              weight:'100'
            }
          ],
          content:'',
          kpiSplitType:"quarterly"
        },
        {
          targetTime:'2',
          kpiSplitItemWeights:[
            {
              targetTime:'4',
              weight:'0'
            },
            {
              targetTime:'5',
              weight:'0'
            },
            {
              targetTime:'6',
              weight:'100'
            }
          ],
          content:'',
          kpiSplitType:"quarterly"
        },
        {
          targetTime:'3',
          kpiSplitItemWeights:[
            {
              targetTime:'7',
              weight:'0'
            },
            {
              targetTime:'8',
              weight:'0'
            },
            {
              targetTime:'9',
              weight:'100'
            }
          ],
          content:'',
          kpiSplitType:"quarterly"
        },
        {
          targetTime:'4',
          kpiSplitItemWeights:[
            {
              targetTime:'10',
              weight:'0'
            },
            {
              targetTime:'11',
              weight:'0'
            },
            {
              targetTime:'12',
              weight:'100'
            }
          ],
          content:'',
          kpiSplitType:"quarterly"
        }
      ]
    };
  },
  computed: {},
  created() {
    this.radio = this.kpiSplitItems[0]['kpiSplitType']
    if (this.kpiSplitItems.length) {
      if (this.kpiSplitItems[0]['kpiSplitType'] == 'month') {
        this.kpiSplitItems.forEach((x,xindex)=>{
          this.monthIndexData.forEach((y,yindex)=>{
            if (x.targetTime == y.targetTime) {
              this.$set(this.monthIndexData,yindex,x)
            }
          })
        })
      } else {
        this.kpiSplitItems.forEach((x,xindex)=>{
          this.quarterIndexData.forEach((y,yindex)=>{
            if (x.targetTime == y.targetTime) {
              this.$set(this.quarterIndexData,yindex,x)
            }
          })
        })
      }
    }
   },
  mounted() {
  },
  methods: {
    handleClose() {
      this.$emit('update:visible', false);
    },
    submit() {
      if (this.radio == 'month') {
        // 校验标准
        // let monthArr = this.monthIndexData.filter(item=>{return !!item.content && item.content.length<5}).map(x=>x.targetTime)
        // if (monthArr.length) {
        //   this.$message.error('如填写标准，必须超过5个字！请完善' + monthArr.join('、') + '月的标准！')
        //   return;
        // }
      } else if (this.radio == 'quarterly') {
        // 校验标准
        // let quarterArr = this.quarterIndexData.filter(item=>{return !!item.content && item.content.length<5}).map(x=>x.targetTime)
        // if (quarterArr.length) {
        //   this.$message.error('如填写标准，必须超过5个字！请完善第' + quarterArr.join('、') + '季度的标准！')
        //   return;
        // }
        // 校验完成占比
        // (decimal * 100).toFixed(2)
        let numberArr = this.quarterIndexData.map(item=> {return item.kpiSplitItemWeights.map(x=>x.weight)})
        // let numberArr = this.quarterIndexData.map(item=> {return item.kpiSplitItemWeights.map(x=>parseInt(x.weight))})
        let numberSumArr = numberArr.map(x=>{return x.reduce((arr,value)=>Number(arr)+Number(value),0)})
        let noEnough = numberSumArr.find(x=>x!=100)
        if (noEnough == 0 || !!noEnough) {
          this.$message.error('每个季度的完成占比总和需要等于100%！')
          return;
        }
      }

      this.$emit('resolveContent',this.radio == 'month' ? this.monthIndexData.filter(x=>x.content != '') : this.quarterIndexData.filter(x=>x.content != ''),this.radio)
      this.handleClose();
    },
  }
};
</script>

<style scoped lang="scss">
.examiner-dialog {
  cursor: default;
  & ::v-deep.el-radio {
    margin-right: 0px;
  }

  ::v-deep.el-tree {
    height: 48vh;
    overflow-y: auto;
  }

  .style-common {
    color: #fff;
    border-radius: 16px;
    background: rgb(47, 194, 91);
    text-align: center;
  }
  .month-style {
    width: 46px;
    height: 18px;
  }
  .quarter-style {
    width: 66px;
    height: 22px;
  }
}

::v-deep .el-dialog__body {
  max-height: 600px;
}
::v-deep .el-table th.el-table__cell.is-leaf {
  background: #f5f7fa !important;
}

</style>
