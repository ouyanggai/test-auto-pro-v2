<!--
 * @Descripttion: 所有表单类型的流程列表多选弹窗
 * @Author: zhengzetao
 * @Date: 2025-01-21
-->
<template>
  <div>
    <slot :viewName="viewName"></slot>
    <el-dialog :visible="visible" :title="'选择'+typeList[fieldType]['name']" width="80%" top="100px" :close-on-click-modal="false" :fullscreen="true"
     :append-to-body="true" class="adjust-department-dialog" @close='handleClose'>
     <div>
      <!-- :placeholder="'查询'+typeList[fieldType]['name']+'名称'" -->
        <el-input style="width:120px;margin-right: 10px;" v-model.trim="searchForm.name" placeholder="标题搜索">
        </el-input>
        <el-button type="primary" @click="searchList">搜索</el-button>

        <el-tabs v-model="activeName" @tab-click="handleClick" size="medium">
          <el-tab-pane label="待办" name="backlog">
            <Backlog v-if="activeName == 'backlog'" ref='backlog' :isAssociateFlow="true" @selectDataEvent="selectDataEvent"/>
          </el-tab-pane>
          <el-tab-pane label="已办" name="finished">
            <Finished v-if="activeName == 'finished'" ref='finished' :isAssociateFlow="true" @selectDataEvent="selectDataEvent" />
          </el-tab-pane>
          <el-tab-pane label="已发" name="submitted">
            <Submitted v-if="activeName == 'submitted'" ref='submitted' :isAssociateFlow="true" @selectDataEvent="selectDataEvent" />
          </el-tab-pane>
        </el-tabs>
        <!-- <div v-if="fieldType == 'flow'">
          <dy-table
            ref="flow"
            :height="'50vh'"
            :fetchData="getList"
            :keys="colKey2"
            :actions="actions2"
            :list="myFlowList"
            :isPagination="true"
            :pagination="pagination"
            :showCheckBox="true"
            @selectDataEvent="selectDataEvent"
          ></dy-table>
        </div> -->
      </div>

      <span slot="footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="confirm">确 定</el-button>
      </span>
    </el-dialog>

  </div>
</template>

<script>
import Api from '@/api';
import mixin from './mixin.js';
import { approveManageFlowStatus, deepClone } from '@/utils';
import DyTable from '@/components/DyTable';
// import Backlog from '@/views/GroupApproveManage/Backlog/index.vue';
// import Submitted from '@/views/GroupApproveManage/Submitted/index.vue';
// import Finished from '@/views/GroupApproveManage/Finished/index.vue';

export default {
  name: 'FormCommonFlowPostList',
  components: {
    DyTable,
    Backlog: () => import('@/views/GroupApproveManage/Backlog/index.vue'),
    Finished: () => import('@/views/GroupApproveManage/Finished/index.vue'),
    Submitted: () => import('@/views/GroupApproveManage/Submitted/index.vue')
  },
  model: {
    prop: 'myValue', // value
  },
  mixins:[mixin],
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    myValue: { // value
      type: [String], // [Array, String, Number]
      default() {
        return '';
      }
    },
    fieldSelectType: {
      type: String,
      default: 'flow'
    }
  },
  data() {
    return {
      // 通用数据
      viewName:'',
      rowData:{},
      typeList: {
        'flow': {
          name:'流程',
        },
      },
      myFlowList: [],
      searchForm:{
        name:'',
        number:'',
      },
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      // 流程数据
      colKey2: {
        name: {
          label: '标题',
          showTooltip: true,
          minWidth: '280',
        },
        flowName: {
          label: '流程名称',
          showTooltip: true,
          minWidth: '150',
        },
        createDate: {
          label: '发起时间',
          minWidth: '160',
          showTooltip: true
        },
        status: {
          label: '流程状态',
          minWidth: '100',
          handle: (scope, createElement) => {
            return createElement('span', approveManageFlowStatus(scope.row.status));
          }
        },
      },
      actions2: [
        {
          label: '详情',
          width: '150',
          actionFixed:'right',
          action: row => {
            this.clickRow = row;
            this.previewHandle(row, false);
          }
        },
      ],
      selectData:[],

      activeName: 'backlog',
    };
  },
  computed: {
    fieldType(){
      let copyType = JSON.parse(JSON.stringify(this.fieldSelectType))
      // console.log('copyType',copyType)
      var str = '';
      for (var item in this.typeList){
        var result = new RegExp(item,'i').test(copyType);
        if (result) {
          str = item;
          break;
        }
      }
      // console.log('str',str)
      return str
    }
  },
  watch: {
    // 后端优化接口查询暂时注释，好像没用
    // "pagination.pages": async function(newVal, oldVal){
    //   this.myFlowList = await this.getList();
    // },
    // "pagination.size": async function(newVal, oldVal){
    //   this.myFlowList = await this.getList();
    // }
  },
  created() {
  },
  mounted() {
    this.init();
  },
  methods: {
    // selectDataEvent(val){
      // if(val.length)this.batchButtonDisable = false
      // else this.batchButtonDisable = true
    // },
    handleClick() {
      this.searchForm.name = '';
      // this.initiator = '';
      // this.flowName = '';
      // this.dateVal = '';
      // this.flowStatus = '';
      // this.flowInstanceName = ''
    },
    selectDataEvent(data){
      this.selectData = data
    },
    handleClose() {
      this.$emit('update:visible', false);
    },
    toggleSelection(arr) {
      if (arr) {
        arr.forEach(row => {
          this.$refs.flow.handleToggleRowSelection(row)
        });
      }
    },
    async init() {
      console.log('init')
      if(!this.fieldSelectType) return;
      // 后端优化接口查询暂时注释，好像没用
      // if (!this.myFlowList.length) {
      //   this.myFlowList = await this.getList();
      // }
    },
    async searchList(){
      // console.log('searchList')
      let query = {
        initiator:'',
        flowName:'',
        queryStartDate:'',
        queryEndDate:'',
        flowInstanceName: this.searchForm.name
        // flowInstanceName:this.flowInstanceName
      }
      if(this.activeName == 'submitted'){
        query.status = ''
        query.name = this.searchForm.name
        // query.name = this.flowInstanceName
        delete query.flowInstanceName
      }
      this.$nextTick(() => {
        this.$refs[this.activeName].pagination.pages = 1;
        this.$refs[this.activeName].queryData(query);
      });
      // this.pagination.pages = 1;
      // this.myFlowList = await this.getList();
    },
    getList() { // 获取列表数据
      console.log('getList')
      // console.log('getList************',JSON.parse(this.myValue),this)
      return new Promise((resolve,reject)=>{
        let url='',data = {};
        if  (this.fieldType == 'flow'){ // 流程
          // console.log('获取流程列表',flowType)
          url = Api.schedule.getFlowInstanceList;
          data = {
            flowName: this.searchForm.name,
            useScope: 'invest',
            statusList:['await_sent','run','withdraw','termination','abandon','rejected','end'],
            flowInstanceBizRelevanceList: [
              {
                otherBiz: 'company',
                otherBizId: this.$store.state.user.companyId
              }
            ],
          };
        }
        this.$axios.post(
          url,
          {
            data,
            pagination: true,
            pages: this.pagination.pages,
            size: this.pagination.size
          },
          res => {
            if (res.isSuccess) {
              if (this.fieldType == 'flow'){
                this.pagination.total = res.total || 0;
                resolve(res.data || [])
              }
            } else {
              this.$message.error(res.message);
            }
          }
        );

      })
    },
    confirm() {
      console.log('this.selectData',this.selectData)
      // return;
      let newSelectData = this.selectData.map(x=>({
        // name:x.name,
        name:x.flowInstanceName || x.name,
        id: x.flowInstanceId || x.id,
        rowData:JSON.stringify(x)
      }))
      this.$emit('selectFlow', {
        flowList: newSelectData,
      });
      // return;
      this.handleClose();
    },

    // 获取表单流程数据（传多条业务id获取数据）
    getAllFormFlowDataByBizId(id) {
      console.log('getAllFormFlowDataByBizId')
      let data = {
        useScope: 'invest',
        auditWayList: [],//this.sFlowTypeList,
        flowInstanceBizRelevanceList: [
          {
            otherBiz: 'contract_seal_review',
            otherBizId: id
          },
        ],
        initiator:"all"
      };
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          {
            data,
            pagination: false,
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data)
            } else {
            }
          }
        )
      })
    },

  }
};

</script>
<style lang='scss' scoped>
.adjust-department-dialog {
  // .dialog-container {
  //   // height: 600px;
  //   height: 48vh;
  //   overflow-y: auto;
  // }

  & ::v-deep.el-radio {
    margin-right: 0px;
  }

  & ::v-deep {
    .el-dialog.is-fullscreen .el-dialog__body {
      max-height: 84vh;
    }
    .el-dialog__body{
      padding:0px 20px !important;
    }
    .el-tabs__header{
      margin: 0px;
    }
    .el-dialog__footer {
      padding-top:30px;
    }
    .dytable-view-container {
      padding: 0px;
    }
  }
}


</style>
