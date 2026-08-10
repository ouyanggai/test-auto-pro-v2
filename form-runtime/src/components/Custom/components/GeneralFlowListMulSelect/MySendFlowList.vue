<!--
 * @Descripttion: 所有表单类型的流程列表多选弹窗
 * @Author: zhengzetao
 * @Date: 2024-08-02
-->
<template>
  <div>
    <slot :viewName="viewName"></slot>
    <el-dialog :visible="visible" :title="'选择'+typeList[fieldType]['name']" width="80%" top="100px" :close-on-click-modal="false"
     :append-to-body="true" class="adjust-department-dialog" @close='handleClose'>
     <div>
        <el-input style="width:200px;margin-right: 10px;" v-model.trim="searchForm.name" :placeholder="'查询'+typeList[fieldType]['name']+'标题'"></el-input>
        <el-input style="width:200px;margin-right: 10px;" v-model.trim="searchForm.flowName" :placeholder="'查询'+typeList[fieldType]['name']+'名称'"></el-input>
        </el-input>
        <el-button type="primary" @click="searchList">查询</el-button>

        <div v-if="fieldType == 'flow'">
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
        </div>
      </div>

      <span slot="footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button type="primary" @click="confirm">确 定</el-button>
      </span>
    </el-dialog>

    <!-- 流程-查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :isReInitiate="isReInitiate" :flowId="flowId"
    :formId="formId" :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId"
    :visible.sync="examineDialogVisible" :isInitiator="true" :selectFlowType="selectFlowType" :businessId="businessId" />

    <!-- 查看流程 -->
    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
    :flowInstanceId="flowInstanceId" :flowId="flowId" :initiatorId="initiatorId"></CheckFlowNodeDetail>


  </div>
</template>

<script>
import Api from '@/api';
import mixin from './mixin.js';
import { approveManageFlowStatus, deepClone } from '@/utils';
import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog';
import CheckFlowNodeDetail from '@/views/GroupApproveManage/components/CheckFlowNodeDetail.vue';
import DyTable from '@/components/DyTable';

export default {
  name: '',
  components: {DyTable,EnterpriseExamineDialog,CheckFlowNodeDetail},
  model: {
    prop: 'myValue', // value
    // event: 'changeMyValue' // input
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
      default: ''
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
        flowName:'',
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
      selectData:[]
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
    visible(val){
      if (val) {
        // console.log('visible-流程列表',val)
        this.$nextTick(x=>{
          if (this.$refs[this.fieldType]){
            this.$refs[this.fieldType].doLayout(); // 解决宽度偶尔缩小
          }

          let getMyNewVal = JSON.parse(this.myValue);
          if (getMyNewVal.flowList) {
            let selectTableIdList = getMyNewVal.flowList.map(x=>x.id);
            let selectFlowList = [];
            selectTableIdList.forEach(x=>{
              selectFlowList.push(this.myFlowList.find(y=>y.id == x))
            })
            // console.log('selectFlowList',selectFlowList)
            this.toggleSelection(selectFlowList);
          }
        })
      }
    },
    async myValue(newVal, oldVal) {
      // console.log('w====atch',newVal)
      if (!this.myFlowList.length) {
        this.myFlowList = await this.getList();
      }
    },
    "pagination.pages": async function(newVal, oldVal){
      this.myFlowList = await this.getList();
    },
    "pagination.size": async function(newVal, oldVal){
      this.myFlowList = await this.getList();
    }
  },
  created() {
  },
  mounted() {
    this.init();
  },
  methods: {
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

      // console.log('this.myFlowList',this.myFlowList)
      if (!this.myFlowList.length) {
        this.myFlowList = await this.getList();
      }
    },
    async searchList(){
      // console.log('searchList')
      this.pagination.pages = 1;
      this.myFlowList = await this.getList();
    },
    getList() { // 获取列表数据
      console.log('getList************',JSON.parse(this.myValue),this)
      return new Promise((resolve,reject)=>{
        let url='',data = {};
        if  (this.fieldType == 'flow'){ // 流程
          console.log(223344,this.myValue)
          let flowType = JSON.parse(this.myValue).flowType;
          console.log('获取流程列表',flowType)
          url = Api.schedule.getFlowInstanceList;
          data = {
            name: this.searchForm.name, // 标题
            flowName: this.searchForm.flowName, // 流程名称
            useScope: 'invest',
            auditWayList: [flowType],
            statusList:['await_sent','run','withdraw','termination','abandon','rejected','end'],
          };

          if (flowType == 'ticket_collection_register') { // 收票登记单
            data.flowInstanceBizRelevanceList = [
              {
                otherBiz: 'company',
                otherBizId: this.$store.state.user.companyId
              }
            ];
            data.initiator = 'all';
          }
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
      // console.log('this.selectData',this.selectData)
      let newSelectData = this.selectData.map(x=>({
        name:x.name,
        id:x.id,
        rowData:JSON.stringify(x)
      }))
      this.$emit('selectFlow', JSON.stringify({
        flowList: newSelectData,
        flowType: JSON.parse(this.myValue).flowType ? JSON.parse(this.myValue).flowType :''
      }));
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
  .dialog-container {
    // height: 600px;
    height: 48vh;
    overflow-y: auto;
  }

  & ::v-deep.el-radio {
    margin-right: 0px;
  }
}
</style>
