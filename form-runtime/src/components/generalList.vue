<!-- 通用列表 -->
<template>
  <div>
    <el-dialog :title="config.title" :visible="visible" append-to-body :close-on-click-modal="false" center
      @close='handleClose' :fullscreen="true">
      <div class="search" v-if="config.search && config.search.length">
        <template v-for="(val,index) in config.search">
          <el-input style="width:120px;margin-right:5px" clearable v-model.trim="query[val.prop]" @keyup.enter.native="getList"
            :placeholder="val.label" @clear="getList" v-if="val.type == 'input'">
          </el-input>
          <el-select v-model.trim="query[val.prop]" :placeholder="val.label"  style="margin-right:5px" clearable
            @change="getList" v-if="val.type == 'select'">
            <el-option
              v-for="item in companyList"
              :key="item.id" :label="item.name" :value="item.id">
            </el-option>
          </el-select>
        </template>
        <el-button type="primary" @click="getList">查询</el-button>
      </div>
      <dy-table :fetchData="fetchData" :actions="actionKey" :keys="config.colKey" :list='tableData'
        :isPagination="true" :pagination="pagination" :showRadio="!config.multi" :showCheckBox="config.multi" ref="dyTable" ></dy-table>
      <div slot="footer" class="dialog-footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button @click="confirm" type="primary">确 认</el-button>
      </div>
    </el-dialog>
    <!-- 查看弹窗(对formMakiing制作的表单的查看) -->
    <EnterpriseExamineDialog v-if="examineDialogVisible" :btnVisible="btnVisible" :isExamine="isExamine" :isReInitiate="false" :flowId="flowId"
      :formId="formId" :flowNodeProxyId="flowNodeProxyId" :jobTaskId="jobTaskId" :flowInstanceId="flowInstanceId" :selectFlowType="selectFlowType"
      :visible.sync="examineDialogVisible" :businessId="businessId" :companyId="companyId"/>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable'
// import EnterpriseExamineDialog from '@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue';
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
const EnterpriseExamineDialog  = () => import('@/views/GroupApproveManage/components/EnterpriseExamineDialog.vue'); //循环引用报错的问题
export default {
  name: 'GeneralList',
  components: { DyTable,EnterpriseExamineDialog },
  props: {
    config: {
      type: Object,
      default() {
        return {
          title: '',
          api: '',
          param: {},
          colKey: {},
          field:'',
          auditWay:"assets_buy_apply",
          pageKey:'pages',
          multi:false,
          search:[],
          isAction:true
        }
      }
    },
    visible: {
      type: Boolean,
      default: false
    },
  },
  data() {
    return {
      tableData: [],
      pagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      actionKey:[
        {
          label: '详情',
          width: '150',
          actionFixed:'right',
          action: row => {
            this.previewHandle(row);
          }
        }
      ],
      flowId: '', // 绑定的业务id
      flowInstanceId: '', // 流程实例id
      isExamine: false,
      btnVisible: false,
      examineDialogVisible: false,
      formId: '',
      flowNodeProxyId: '',
      jobTaskId: '',
      businessId:'',
      selectFlowType:'',
      companyId:'',
      query:{},
      companyList:[]
    };
  },
  created() {
    if(!this.config.isAction)this.actionKey = []
    if(this.config?.search?.length){
      this.config.search.forEach(el=>{
        this.$set(this.query,el.prop,'')
        if(el.option == 'companyList'){
          console.log('el.option',el.option)
          this.getCompanyList()
        }
      })
    }
  },
  mounted() { },
  watch: {},
  computed: {},
  methods: {
    // 获取公司列表
    getCompanyList() {
      this.$axios.post(
        Api.budgetManage.getParentCompanyList,
        {
          data: {
            id:localstorageGet('companyId')
          }
        },
        res => {
          if (res.isSuccess) {
            this.companyList = res.data.map(item=>{
              return {
                id:item.id,
                name:item.name
              }
            });
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    previewHandle(row) {
      this.isExamine = false;
      this.isReInitiate = false;
      this.flowId = row.flowProxyId;
      this.formId = row.formProxyId;
      this.flowNodeProxyId = row.flowNodeProxyId;
      this.flowInstanceId = row.id;
      this.jobTaskId = row.jobTaskId;
      this.selectFlowType = row.auditWay;
      const find = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == row.auditWay);
      this.businessId = find?.otherBizId || '';
      const company = row.flowInstanceBizRelevanceList.find(item => item.otherBiz == 'company');
      this.companyId = company?.otherBizId || '';
      this.examineDialogVisible = true;
    },
    getList(){
      this.pagination.pages = 1
      this.fetchData()
    },
    fetchData() {
      if (this.config.api) {
        let data = this.config.param
        data = {
          ...data,
          ...this.query
        }
        this.$axios.post(
          this.config.api,
          {
            data,
            pagination: true,
            [this.config.pageKey]: this.pagination.pages,
            size: this.pagination.size

          },
          res => {
            if (res.isSuccess) {
              if(res.data && res.data.dataList !== undefined){
                this.tableData = res.data?.dataList || []
                this.pagination.total = res.data.total;
              }else{
                this.tableData = res.data
                this.pagination.total = res.total
              }
            } else {
              this.$message.error(res.message)
            }
          }
        );
      }
    },
    confirm() {
      //
      if(this.config.multi){ //多选
        let selectDatas = this.$refs.dyTable.selectDatas
        if(selectDatas.length){
          this.$emit('confirmChoose',selectDatas)
          this.handleClose()
        }else{
          this.$message.warning('请选择一条数据')
        }
      }else{//单选
        if(this.$refs.dyTable.currentRowId){
          let find = this.tableData.find(item=>item.id == this.$refs.dyTable.currentRowId)

          if(find.flowInstanceBizRelevanceList){
            let bizData = find.flowInstanceBizRelevanceList.find(item => item.otherBiz == this.config.auditWay)
            let resData = {
              flowId:find.id,
              businessId:bizData.otherBizId,
              businessName:find.flowName || find.formName,
              name:find.flowName || find.formName
            }
            this.$emit('confirmChoose',resData)
          }else{
            this.$emit('confirmChoose',find)
          }
          this.handleClose()
        }else{
          this.$message.warning('请选择')
        }
      }
    },
    handleClose() {
      this.$emit('update:visible', false)
    }
  },
};
</script>
<style lang="scss" scoped>
// ::v-deep .el-dialog {
//   width: 1024px;
//   // .el-dialog__body{
//   //   max-height: 500px;
//   // }
// }
</style>
