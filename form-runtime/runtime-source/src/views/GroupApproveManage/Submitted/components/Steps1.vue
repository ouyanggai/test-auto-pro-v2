<!--
 * @Descripttion: 流程步骤
 * @Author: zhengzetao
 * @Date: 2022-06-10
-->
<template>
  <div class="framework-manage-setting-container">
    <FrameworkTreeSet :treeData="treeData" @clickFrameworkTree="clickFrameworkTree"
      @flowListFromCatch="flowListFromCatch" :companyFlowTemplateType="companyFlowTemplateType" />
    <div class="main-right-panel">
      <el-input style="width:240px;" v-model.trim="serachName" @keyup.enter.native="debounceQueryFlow"
        placeholder="查找流程名称" @input="debounceQueryFlow">
        <i slot="suffix" style="cursor:pointer" @click="debounceQueryFlow" class="el-input__icon el-icon-search"></i>
      </el-input>
      <dy-table :showRadio="true" :keys="colKey" :actions="actions" :fetchData="fetchData" :pagination="pagination" :list="tableData"
        :isPagination="true" :showSelection="false" @rowClick="isRowClick" :height="height" :loading='tableLoading' ref="dytable">
      </dy-table>
    </div>
    <CheckFlowNodeDetail v-if="checkViewFlowDetailVisible" :dialogVisible.sync="checkViewFlowDetailVisible"
     :flowId="flowId" :initiatorId="initiatorId" :isDraft="true"></CheckFlowNodeDetail>
  </div>
</template>

<script>
import Api from '@/api';
import CheckFlowDialog from './CheckFlowDialog';
import DyTable from '@/components/DyTable';
import FrameworkTreeSet from './FrameworkTreeSet';
import CheckFlowNodeDetail from '../../components/CheckFlowNodeDetail.vue';
import { localstorageGet, localstorageSet, localstorageRemove } from '@/utils/auth';

export default {
  name: '',
  components: {
    CheckFlowDialog, FrameworkTreeSet, DyTable, CheckFlowNodeDetail
  },
  props: {
    sFlowTypeList: {
      type: Array,
      default: () => {
        return [];
      }
    },
    flowJson: {
      type: Object,
      default: () => {
        return null;
      }
    }
  },
  data() {
    return {
      formPersonFields: '',
      previewVisible: false,
      checkViewFlowDetailVisible: false,
      formId: '',
      flowId: '',
      tableData: [
        // {
        //   id: 1,
        //   flowName: '费用报销单',
        //   type: 'expense_budget'
        // },
        // {
        //   id: 2,
        //   flowName: '公司预算金额调剂申请单',
        //   type: 'expense_reimbursement'
        // }
      ],
      selectFlowType: '',
      treeData: [],
      treeDataRow: {},
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      colKey: {
        flowName: '流程名称',
        typeName: {
          label: '类型'
        }
        // createDate: '最后更新时间',
        // remark: '备注'
      },
      actions: [
        {
          width: '120px',
          label: '查看流程',
          action: (row) => {
            this.handleCheckFlow(row);
          }
        }
        // {
        //   width: '190px',
        //   label: '编 辑',
        //   action: (row) => {
        //     this.handleToLink(row, 2);
        //   }
        // },
        // {
        //   width: '190px',
        //   label: '停 用',
        //   action: (row) => {
        //     this.handleToLink(row, 2);
        //   }
        // },
        // {
        //   width: '190px',
        //   label: '查 看',
        //   action: (row) => {
        //     this.handleToLink(row, 1);
        //   }
        // },
        // {
        //   width: '190px',
        //   label: '删 除',
        //   action: (row) => {
        //     this.handleToLink(row, 2);
        //   }
        // }
      ],
      companyFlowTemplateType: [],
      serachName: '',
      height:null,
      initiatorId:'' , //发起人
      tableLoading: false
    };
  },
  computed: {},
  watch: {
    flowJson: {
      handler(val) {
        if (val) {
          this.isRowClick({ row: val });
          this.$nextTick(() => {
            this.nextStepHandle();
          });
        }
      },
      immediate: true
    }
  },
  created() {
  },
  mounted() {
    //
    let height = document.querySelector('.main-right-panel').clientHeight
    this.height = height - 120
    // this.fetchData();
    // this.getDepartTree();
    this.getGroupList();
    this.formType();
  },
  methods: {
    debounceQueryFlow(){
      if(this.timer)clearTimeout(this.timer)
      this.timer = setTimeout(()=>{
        this.$refs.dytable.currentRowId = ''
        this.flowId  =''
        this.pagination.pages = 1
        this.fetchData({})
      },300)
    },
    nextStepHandle() {
      this.$emit('nextStep');
    },
    // 查询表单类型，并过滤出需要的类型（公司相关的类型）
    formType() {
      this.$axios.post(
        Api.frameworkInfo.departmentFramework.flow.typeList,
        {
          data: {
            name: '',
            useScope: 'invest'
          },
          pages: 1,
          size: 99999
        },
        res => {
          if (res.isSuccess) {
            this.companyFlowTemplateType = res.data.map(item => item.id);// .filter(x => this.sFlowTypeList.includes(x.auditWay)).map(x => x.id);
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    isRowClick(row) {
      console.log('isRowClick',row)

      // this.clickSelectedRowid = row.row.id;
      this.flowId = row.row.id; // 流程id
      this.formId = ''
      this.flowNodeType = ''
      this.nextNodeName = ''
      this.nextNodeProxyId = ''
      this.formPersonFields = ''
      if(row.row.formTemplateList && row.row.formTemplateList[0] && row.row.formTemplateList[0].id)this.formId = row.row.formTemplateList[0].id; // 表单id
      // 当前节点审批时，如果下个节点设置了审批人自选，那么需要在此处赋值，才能得到提交自选审批人接口所需要的参数
      if (row.row.formTemplateBizRelevanceVo) {
        this.flowNodeType = row.row.formTemplateBizRelevanceVo.flowNextNodeAuditType; // 判断下一个审批节点是否自选
        this.nextNodeName = row.row.formTemplateBizRelevanceVo.nextNodeName; // 下一审批节点名称
        this.nextNodeProxyId = row.row.formTemplateBizRelevanceVo.nextNodeProxyId; // 下一审批节点
        this.formPersonFields = row.row.formTemplateBizRelevanceVo.formPersonFields;// 下一审选择表单人员
      }
      // row.id = res.data[0].id;

      this.$emit('getFlowType', row.row);
    },
    // 获取实施平台配置的流程分组
    getGroupList() {
      this.$axios.post(
        Api.flowManage.getFlowGroupList,
        { data: { bizScope: 'epc', customerCode: this.$store.state.user.customerCode }},
        res => {
          if (res.isSuccess) {
            this.treeData = res.data.filter(i => i.name != 'project_workflow_store');
          }
        }
      );
    },
    /*     getDepartTree() {
      // 获取所在公司和集团公司层级
      this.$axios.post(
        Api.schedule.getMainAndDeputyCompanyList,
        {
          data: {
            id: localstorageGet('userId') // 用户id
          }
        },
        res => {
          if (res.isSuccess) {
            this.treeData = res.data;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    }, */
    clickFrameworkTree(data) {
      this.serachName = '';
      this.pagination.pages = 1;
      this.treeDataRow = data;
      this.fetchData(data);
      // this.departmentId = this.treeDataRow.id;
      // this.companyId = this.treeDataRow.parentId;
    },
    flowListFromCatch(data, mainCompanyData) {
      this.pagination.total = data.total;
      this.pagination.pages = data.page;
      this.tableData = data.catch;
      this.treeDataRow = mainCompanyData;
      // 焚毁缓存
      localstorageRemove('flowSelectList');
    },
    // 查出固定的流程id
    fetchData(data) {
      if (!Object.keys(this.treeDataRow).length) {
        return;
      }
      this.tableLoading = true;
      // console.log('this.treeDataRow.id',this.treeDataRow)
      // return
      const param = {
        data: {
          // typeId: '',
          typeIds: this.companyFlowTemplateType,
          flowName: this.serachName,
          // nextNodeName: '',
          // flowNodeType: '',
          // nextNodeProxyId: '',
          // flowStatus: 'enable',
          // auditWay: row.type,
          useScope: 'invest',
          groupId: data ? data.id : this.treeDataRow.id,
          notAuditWayList: ["staff_annual_assessment"],
        },
        showMe: true, // 只能看到配置了自己为发起人的流程
        ignoreFormTemplateBizRelevanceData: true,
        formTemplateBizRelevanceList: [ // 查询多个条件使用参数，如果关联了项目，里面都要加一个projectId
          // {
          //   otherBiz: 'company',
          //   otherBizId: data && Object.keys(data).length ? data.id : ''
          // },
          // {
          //   otherBiz: 'customerCode',
          //   otherBizId: this.$store.state.user.customerCode
          // }
        ],
        notFormTemplateBizRelevanceList:[
          {otherBiz:'isProject',otherBizId:'isProject'}
        ],
        // formTemplateBizRelevance: { // 查询单个条件使用这个参数
        //   otherBiz: 'customerCode',
        //   otherBizId: this.$store.state.user.customerCode
        // },
        platformCode: '999999', // 如果使用实施管理平台的流程需要加这个字段
        ignoreTemplateData: true,
        pagination: true,
        pages: this.pagination.pages,
        size: this.pagination.size
      };
      if(this.serachName)delete param.data.groupId
      if (this.$store.state.user.groupDepartment != 'group') { // 如果关联了项目，里面都要加一个projectId
        param.formTemplateBizRelevance = {
          projectId: this.$store.state.user.projectId
        };
      }
      if(!param.data.flowName){
        param.data.groupId = this.treeDataRow.id
      }
      this.$axios.post(
        Api.schedule.getFlowTemplateList,
        param,
        (res) => {
          if (res.isSuccess) {
            // if (!res.data.length) {
            //   this.$message.error('请先建立相应流程！');
            // }
            // // else if (res.data.length > 1) {
            // //   this.$message.error('该表单存在多条流程，只允许一条流程！');
            // // }
            // else {
            //   this.flowId = res.data[0].id; // 流程id
            //   // 如果是多分支，没办法在这里获取自选审批人所需要的参数，需要在提交审批时，提交接口会返回，然后再次调起审批弹窗。（后端接口不支持）
            //   if (res.data[0].formTemplateBizRelevanceVo) {
            //     this.flowNodeType = res.data[0].formTemplateBizRelevanceVo.flowNextNodeAuditType; // 判断下一个审批节点是否自选
            //     this.nextNodeName = res.data[0].formTemplateBizRelevanceVo.nextNodeName; // 下一审批节点名称
            //     this.nextNodeProxyId = res.data[0].formTemplateBizRelevanceVo.nextNodeProxyId; // 下一审批节点
            //   }
            //   row.id = res.data[0].id;
            //   this.$emit('getFlowType', row);
            // }

            this.tableData = res.data || [];
            const catchData = {
              page: param.pages,
              total: res.total,
              companyId: this.treeDataRow.id,
              catch: res.data
            };
            localstorageSet('flowSelectList', catchData);
            this.pagination.total = res.total;
          }
          this.tableLoading = false;
        }
      );
    },
    radioChange(value, row) {
      this.fetchData(row);

      // if (row.formTemplateList && row.formTemplateList.length) {
      //   this.formId = row.formTemplateList[0].id;
      //   this.nextNodeName = row.formTemplateBizRelevanceVo.nextNodeName;
      //   this.flowNodeType = row.formTemplateBizRelevanceVo.flowNextNodeAuditType;
      //   this.nextNodeProxyId = row.formTemplateBizRelevanceVo.nextNodeProxyId;
      // }
    },
    previewHandle(row) {
      this.selectFlowType = row.type;
      this.previewVisible = true;
      // this.flowId = row.id;
      // if (row.formTemplateList && row.formTemplateList.length) {
      //   this.formId = row.formTemplateList[0].id;
      //   this.previewVisible = true;
      // }
    },
    handleCheckFlow(row) {
      console.log('handleCheckFlow',row)
      // 查看流程
      this.flowId = row.id;
      this.initiatorId = this.$store.state.user.userId
      this.checkViewFlowDetailVisible = true;
    }
  }
};
</script>

<style scoped lang="scss">
.container {
  height: 100%;
}

.framework-manage-setting-container {
  height: 100%;
  padding: 14px;
  cursor: default;

  .main-right-panel {
    margin-left: 180px;
    border-left: 2px solid #e4e7ed;
    height: 100%;
    overflow-y: hidden;
    padding-left: 20px;
    padding-top: 20px;
    background-color: #fff;
  }
}

.el-radio {
  width: 90%;
  white-space: nowrap;
  text-overflow: ellipsis;
  overflow: hidden;
  line-height: 28px;
  align-items: center;
}

.el-radio-group {
  width: 90%;
}
::v-deep .dytable-view-container .dytable-view-paging{
  margin-top:-10px !important;
}
</style>
