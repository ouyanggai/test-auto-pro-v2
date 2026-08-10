<!--
 * @description:目标责任书指标弹框
 * @Author: Calvin
 * @Date: 2022-03-31 16:57:40
 * @FilePath: \src\views\TaskManage\TaskArrange\components\WorkTargetDailog.vue
-->
<template>
  <el-dialog
    :visible="visible"
    title="目标责任书指标"
    width="80%"
    :close-on-click-modal="false"
    class="adjust-department-dialog"
    @close='handleClose'
  >
    <div class="work-target-container">
      <div class="left-wrap">
        <ul style="list-style:none">
          <li>目标责任书名称</li>
          <li
            v-for="(item,index) in workList"
            :key="index"
          >
            <el-radio
              class="radio-wrap"
              v-model="workItemRadio"
              :label="item.id"
              :title="item.title"
              @change="chooseTargetBookName($event,item)"
            >{{item.title}}</el-radio>
          </li>
        </ul>
      </div>
      <div class="right-wrap">
        <div style="margin-bottom:10px;font-weight: 600;">目标责任书内容</div>
        <el-table
          :data="workDetailList"
          ref="table"
          border
          align="center"
          :highlight-selection-row="'true'"
          :expand-row-keys="expandRowKeys"
          :row-key="getRowKeys"
          style="width: 100%"
          @selection-change="handleSelectionChange"
          @expand-change="handleExpandChange"
        >
          <el-table-column
            type="selection"
            width="55"
          >
          </el-table-column>
          <el-table-column type="expand" width="1">
            <template slot-scope="props">
              <el-table
                ref="kpiResolveTable"
                stripe
                align="center"
                highlight-current-row
                @current-change="handleCurrentChange"
                :data="kpiResolveList"
                style="width: 100%"
                v-if="kpiResolveList.length"
              >
                <el-table-column
                  prop="timeStr"
                  label="时间">
                </el-table-column>
                <el-table-column
                  prop="content"
                  label="完成标准">
                </el-table-column>
              </el-table>
            </template>
          </el-table-column>
          <el-table-column label="目标项目(一级)">
            <template slot-scope="scope">
              <el-select
                disabled
                v-model="scope.row.indicatorsType.id"
                placeholder="请选择"
              >
                <el-option
                  v-for="item in targetList"
                  :key="item.id"
                  :label="item.name"
                  :value="item.id"
                >
                </el-option>
              </el-select>
            </template>
          </el-table-column>
          <el-table-column
            v-if="manageType =='work_target'"
            label="目标项目(二级)"
          >
            <template slot-scope="scope">
              <el-input
                type="textarea"
                size="medium"
                readonly
                :autosize="{ minRows: 1 }"
                v-model.trim="scope.row.targetItemTwo"
              >
              </el-input>
            </template>
          </el-table-column>
          <el-table-column label="具体目标项目内容">
            <template slot-scope="scope">
              <el-input
                type="textarea"
                size="medium"
                readonly
                :autosize="{ minRows: 1 }"
                v-model.trim="scope.row.content"
              >
              </el-input>
            </template>
          </el-table-column>
          <el-table-column
            label="权重"
            width="100"
          >
            <template slot-scope="scope">
              <el-input
                size="mini"
                readonly
                v-model.trim="scope.row.weight"
              ></el-input>
            </template>
          </el-table-column>
          <el-table-column label="目标完成标准(考核标准)">
            <template slot-scope="scope">
              <el-input
                type="textarea"
                size="medium"
                readonly
                :autosize="{ minRows: 1 }"
                v-model.trim="scope.row.assessmentMethod"
              >
              </el-input>
            </template>
          </el-table-column>
          <el-table-column label="目标完成时间节点">
            <template slot-scope="scope">
              <el-input
                type="textarea"
                size="medium"
                readonly
                :autosize="{ minRows: 1 }"
                v-model.trim="scope.row.assessmentTime"
              >
              </el-input>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
    <span slot="footer">
      <el-button @click="handleClose">取 消</el-button>
      <el-button
        type="primary"
        @click="handleSubmit"
      >确 定</el-button>
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
    selectManageId: {
      type: String,
      default: ''
    },
    userId: {
      type: String,
      default: ''
    },
    selectManageContentId: {
      type: String,
      default: ''
    },
    selectContentResolveId: {
      type: String,
      default: ''
    },
    selectResolveKpiId: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      kpiResolveCurrentRow:null,
      selectKpiList:[],
      kpiResolveList:[],
      expandRowKeys: [],
      workList: [],
      manageId: '',
      workDetailList: [],
      workItemRadio: '',
      manageType: 'work_target',
      targetList: [],
      selectDatas: []
    };
  },
  computed: {},
  watch: {
    'kpiResolveList':{
      deep:true,
      handler:function(val){
        console.log('watch kpiResolveList')
        // 编辑回显时高亮分解的指标表格
        let that = this;
        if (this.selectResolveKpiId || this.selectContentResolveId) {
          val.forEach(x=>{
            if (x.kpiSplitType == 'quarterly') {
              if (x.kpiSplitItemWeights[0]['id'] == this.selectResolveKpiId) {
                this.$nextTick(j=>{ // $nextTick
                  // console.log('watch',this.$refs.kpiResolveTable)
                  that.$refs.kpiResolveTable.setCurrentRow(x);
                })
              }
            } else {
              // console.log('this.selectContentResolveId',this.selectContentResolveId)
              if (x.id == this.selectContentResolveId) {
                // console.log('x',x)
                this.$nextTick(j=>{
                  // console.log('watch',this.$refs.kpiResolveTable)
                  that.$refs.kpiResolveTable.setCurrentRow(x);
                })
              }
            }
            
          })
          // console.log('kpiObj',kpiObj)
          // this.$refs.kpiResolveTable.setCurrentRow(item.kpiSplitItems[0]);
        }
        
      }
    }
  },
  created() { },
  mounted() {
    this.getCurrentKpiGroup();
  },
  methods: {
    //获取row的key值（这个方法不用修改，直接放上去就行）
    getRowKeys(row) {
        return row.id; //id为table每一栏的id
    },
    // 获取目标责任书名称
    getCurrentKpiGroup() {
      const data = {
        // userId:this.userId
        user:{
          id: this.userId
        }
      };
      this.$axios.post(
        Api.taskManage.taskArrange.getCurrentKpiGroup,
        {
          data
        },
        res => {
          if (res.isSuccess) {
            this.workList = res.data;
            if (this.selectManageId) {
              this.workList.forEach(item => {
                if (item.id == this.selectManageId) {
                  this.workItemRadio = item.id;
                  this.chooseTargetBookName(item.id, item);
                }
              });
            }
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 一级目标项目
    getIndicatorsTypeList() {
      const params = {
        data: {
          // enableType: 'enable',
          manageType: this.manageType
        }
      };

      this.$axios.post(
        Api.performance.indicatorsTypeList,
        params,
        res => {
          if (res.isSuccess) {
            this.targetList = res.data ? res.data : [];
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    chooseTargetBookName(id, item) {
      this.manageType = item.manageType;
      this.manageId = item.id;
      this.getIndicatorsTypeList();
      this.getKpiGroupDetail(id);
    },
    // 获取目标责任书内容
    getKpiGroupDetail(id) {
      // console.log('getKpiGroupDetail')
      this.$axios.post(
        Api.taskManage.taskArrange.getWorkTargetDetail,
        {
          data: {
            id
          }
        },
        res => {
          if (res.isSuccess) {
            this.workDetailList = res.data.keyPerformanceIndicatorsList;
            // console.log('this.workDetailList',this.workDetailList)
            // console.log('=====kpiResolveList',this.kpiResolveList)
            this.workDetailList.forEach(item => {
              if (item.id == this.selectManageContentId) {
                console.log('item',item)
                this.$nextTick(() => {
                  this.$refs.table.clearSelection();
                  this.$refs.table.toggleRowSelection(item);
                  this.selectDatas = [item];
                });
              }
            });
            
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    // 目标责任书内容列表选择
    handleSelectionChange(data) {
      // console.log('handleSelectionChange',data)
      this.kpiResolveCurrentRow = null;
      this.kpiResolveList = [];
      if (data.length > 1) {
        const newArr = data[1];
        data.shift();
        this.$refs.table.clearSelection();
        this.$refs.table.toggleRowSelection(newArr);

        
      } else if (data.length == 1){
        this.expandRowKeys = [data[0]['id']]
        // this.selectKpiList = data[0];
        // this.kpiResolveList = data[0]['kpiSplitItems']

        if (data[0]['kpiSplitItems']) {
          // 拼接季度和月度指标
          data[0]['kpiSplitItems'].forEach((x,xIndex)=>{
            if (x.kpiSplitType == 'quarterly') {
              x.kpiSplitItemWeights.forEach((y,yIndex)=>{
                if (y.weight > 0) {
                  // x.timeStr = '第'+x.targetTime +'季度-'+ y.targetTime + '月 ('+ y.weight +')';
                  this.kpiResolveList.push({
                    timeStr:'第'+x.targetTime +'季度-'+ y.targetTime + '月 ('+ y.weight +'%)',
                    content: x.content,
                    id:x.id,
                    kpiSplitItemWeights:[{
                      id:y.id
                    }]
                  })
                }
              })
            } else if (x.kpiSplitType == 'month'){
              if (x.content) {
                // x.timeStr = x.targetTime + '月';
                this.kpiResolveList.push({
                  timeStr: x.targetTime + '月',
                  content: x.content,
                  id:x.id
                })
              }
            }
          })
        }
        // console.log('kpiResolveList',this.kpiResolveList)
      } else {
        this.expandRowKeys = []
      }
      this.selectDatas = data;
    },
    handleExpandChange(row,data){
      console.log(row)
      console.log(data)
    },
    //指标分解列表高亮选择
    handleCurrentChange(val) {
      console.log('val',val)
      this.kpiResolveCurrentRow = val;
    },
    handleSubmit() {
      console.log('handleSubmit')
      // 有分解指标又没有选择分解的指标，进行提示
      if (!!this.kpiResolveList.length && !this.kpiResolveCurrentRow) {
        this.$message.error('请选择已分解指标！')
        return;
      }
      console.log('handleSubmit',2)
      // let data = {
        //     kpiGroup:{
        //       id:this.manageId,//年度目标责任书id,必填
        //       keyPerformanceIndicatorsList:[{
        //         id:1,//考核指标项id，必填
        //         kpiSplitItems:[{
        //             id:2,//指标分解项id，有指标分解就必填，没有kpiSplitItems字段就为null
        //             kpiSplitItemWeights:[{
        //               id:3//当指标分解类型为“季度”时，这个必填
        //           }]
        //         }]
        //       }] 
        //     }
        //   }

      let newSelectDatas = {
        // kpiGroup:{
          id:this.manageId,//年度目标责任书id,必填
          keyPerformanceIndicatorsList:[{
            id:this.expandRowKeys[0],//考核指标项id，必填
            kpiSplitItems:this.kpiResolveCurrentRow?[this.kpiResolveCurrentRow]:null
          }] 
        // }
      }
      console.log('newSelectDatas1',newSelectDatas)
      // return;

      this.$emit('workTargetSelect', {
        manageType: this.manageType,
        manageId: this.manageId,
        selectDatas: this.selectDatas,
        newSelectDatas:newSelectDatas
      });
      this.handleClose();
    },
    handleClose() {
      this.$emit('update:visible', false);
    }
  }
};
</script>

<style scoped lang="scss">
::v-deep .el-table__header .el-checkbox {
  visibility: hidden;
}

::v-deep .el-table__expand-icon {
  display: none;
}

::v-deep .el-table__body tr.current-row>td.el-table__cell {
    background-color: rgb(179, 216, 255) !important;
  }
// ::v-deep .el-table--enable-row-hover .el-table__body tr:hover>td {
//     background-color: #ecf5ff !important;
//   }
.work-target-container {
  display: flex;
  min-height: 400px;
  user-select: none;
  .left-wrap {
    min-width: 170px;
    // min-height: 400px;
    border-right: 1px solid #f2f2f2;
    li {
      padding-left: 10px;
      line-height: 40px;
      border-bottom: 1px solid #f2f2f2;
      &:first-child {
        // text-align: left;
        // cursor: default;
        padding-left: 10px;
        font-weight: 600;
      }
    }
    .radio-wrap {
      width: 160px;
      overflow: hidden;
      display: inline-block;
      text-overflow: ellipsis;
      vertical-align: middle;
    }
  }
  .right-wrap {
    flex: 1;
    padding: 10px 22px;
  }
}


</style>
