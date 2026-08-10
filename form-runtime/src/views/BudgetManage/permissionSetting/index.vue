<template>
  <div class="permissionSetting">
    <div class="top">
      <SelectTree :options="userList" :value="userId" :props="defaultProps" :clearable="true" labelName="请选择用户" style="width: 300px;"
            isLevel @getValue="selsectUser($event,'search')" @clear="userId=''" />
      <el-button type="success" @click="search" style="margin-left: 20px;">搜 索</el-button>
      <el-button type="primary" @click="add('')">新 增</el-button>
      <el-button type="danger" @click="deleteData">删 除</el-button>
    </div>
    <div class="content">
      <div class="table-content">
        <el-table :data="tableData" stripe  @selection-change="handleSelectionChange">
          <el-table-column
            type="selection"
            align="center"
            width="45">
          </el-table-column>
          <el-table-column prop="userName" label="姓名" width="100">
            <template slot-scope="scope">
                {{ scope.row.fullUserInfoVo.name }}
            </template>
          </el-table-column>
          <el-table-column prop="budgetYear" label="所在公司" width="200">
            <template slot-scope="scope">
                {{ scope.row.fullUserInfoVo.company.name }}
            </template>
          </el-table-column>
          <el-table-column prop="totalBudget" label="所在部门" width="200">
            <template slot-scope="scope">
                {{ scope.row.fullUserInfoVo.department.departmentName }}
            </template>
          </el-table-column>
          <el-table-column prop="purview" label="查看权限范围" show-overflow-tooltip>
          </el-table-column>
          <el-table-column label="操作" width="80" align="center">
            <template slot-scope="scope">
                <el-button @click="add(scope.row)" type="text" size="small">分配权限
                </el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination :page-sizes="[10, 20, 50, 100]" background :total="pagination.total"
          :current-page="pagination.currentPage" layout="total, sizes, prev, pager, next" @size-change="handlePageSize"
          @current-change="pageChange" style="text-align: center; margin-top: 15px"></el-pagination>
      </div>
    </div>
    <el-dialog title="新增权限" :visible="addVisible" width="50%" :close-on-click-modal="false" lock-scroll
      :before-close="handleClose" top="100px">
      <el-form :model="form" label-width="140px" :rules="rules" ref="ruleForm">
        <el-form-item label="请选择用户" prop="userId">
          <SelectTree :options="userList" :value="form.userId" :props="defaultProps" :clearable="true" labelName="请选择用户"
            isLevel @getValue="selsectUser($event)" @clear="clear('user')" />
        </el-form-item>
        <el-form-item label="请分配数据权限" prop="userBusinessVoList">
          <SelectTree :options="departMentList" :isCheck="true" ref="dutySelectTree" :value="form.userBusinessVoList" v-if="addVisible"
            @setValue="selectData" :props="defaultProps" :isShowChildTitle="true" :clearable="true" labelName="岗位" @clear="clear('duty')" />
        </el-form-item>
      </el-form>
      <span slot="footer" class="dialog-footer">
        <el-button  @click="handleClose('close')">关 闭</el-button>
        <el-button type="primary" @click="handleClose('save')" :loading="submitLoading">保 存</el-button>
      </span>
    </el-dialog>
  </div>
</template>
<script>
import SelectTree from "@/components/selectTree";
export default {
  components: { SelectTree },
  data() {
    return {
      query: {
        userId: "",
        type: "",
      },
      defaultProps: {
        children: "childrenList",
        label: "name",
        value: "id",
      },
      companyList: [],
      departMentList:[],
      userList: [],
      tableData: [],
      pagination: {
        total: 0,
        currentPage: 1,
        pageSize: 10,
      },
      addVisible: false,
      form: {
        userId: "",
        userBusinessVoList: null
      },
      rules: {
        userId: [{ required: true, message: "请选择用户", trigger: "change" }],
        userBusinessVoList: [
          { required: true, message: "请分配数据权限", trigger: "change" },
        ],
      },
      submitLoading:false,
      multipleSelection:[],
      userId:''
    };
  },
  mounted() {
    this.getList();
    this.getCompany();
  },
  methods: {
    search(){
      this.getList();
    },
    deleteData(){
      if(this.multipleSelection.length>0){
        this.$confirm('是否确认删除？', '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        }).then(() => {
          this.$axios.post(
          '/web/measuring/api/costJurisdiction/delete',
          {
            data: {

            },
            ids:this.multipleSelection.map(item=>item.id)
          },
          (res) => {
            if (res.isSuccess) {
              this.$message.success('删除成功')
              this.getList()

            } else {
              this.$message.error(res.message);
            }
          }
        );
        })
      }else{
        this.$message.warning("请选择要删除的数据")

      }
    },
    handleSelectionChange(val){
      this.multipleSelection = val;
    },
    add(row) {
      this.addVisible = true;
      if(row){
        this.getDetial(row.id)
      }
    },
    getDetial(id){
      this.$axios.post(
          "/web/measuring/api/costJurisdiction/findById",
          {
            data: {
              id
            },
          },
          (res) => {
            if (res.isSuccess) {
              // this.$message.success("保存成功")
              // this.getList()
              // this.cancel()
              const userBusinessVoList =[]
              res.data.userBusinessVoList.map(item=>{
                userBusinessVoList.push(item.departmentId||item.companyId)
              })
              this.form ={
                id:res.data.id,
                userId:res.data.userId,
                purview:res.data.purview,
                userBusinessVoList:userBusinessVoList
              }
              this.$nextTick(()=>{
                const defaultExpandedKey = []
                userBusinessVoList.map(item=>{
                  // this.$refs.dutySelectTree.$refs.selectTreeCon.getNode(item)
                  console.log(this.$refs.dutySelectTree.$refs.selectTreeCon.getNode(item),'this.$refs.dutySelectTree.$refs.selectTreeCon.getNode(item)')
                  defaultExpandedKey.push(this.$refs.dutySelectTree.$refs.selectTreeCon.getNode(item).data)
                })
                console.log(defaultExpandedKey,'defaultExpandedKey+++')
                this.$refs.dutySelectTree.$refs.selectTreeCon.defaultExpandedKey = defaultExpandedKey
                // this.$refs.selectTreeCon.getNode(item).data
                this.$refs.dutySelectTree.$refs.selectTreeCon.setCheckedKeys(userBusinessVoList)
                console.log(this.$refs.dutySelectTree,'this.$refs.dutySelectTree')
              })
            } else {
              this.$message.error(res.message);
            }
          }
        );
    },
    handleClose(type) {
      console.log(this.form, "999999");
      if(type == "save"){
        let purview =''
        const userBusinessVoList = []
        this.form.userBusinessVoList.map((item,index)=>{
          purview += ((index!=0?',':'')+(item.fullName||item.name))
          userBusinessVoList.push({companyId:item.type=='1'?item.id:item.parentId,departmentId:item.type=='2'?item.id:'',type:item.type})
        })
        this.submitLoading = true
        this.$axios.post(
          this.form.id?'/web/measuring/api/costJurisdiction/update':"/web/measuring/api/costJurisdiction/save",
          {
            data: {
              userId: this.form.userId,
              userBusinessVoList: userBusinessVoList, //1 公司 2 部门
              purview:purview,
              id:this.form.id||''
            },
          },
          (res) => {
            this.submitLoading = false
            if (res.isSuccess) {
              this.$message.success("保存成功")
              this.getList()
              this.cancel()
            } else {
              this.$message.error(res.message);
            }
          }
        );
      }else{
       this.cancel()
      }
      
    },
    cancel(){
      this.form={
        userId: "",
        userBusinessVoList: null,
        purview:''
      }
      this.$refs.ruleForm.resetFields()
      this.addVisible = false
     
    },
    selsectUser(data,type) {
      console.log(data, "5555");
      if(type){
        this.userId = data.id;
      }else{
        this.form.userId = data.id;
      }
      
    },
    selectData(data) {
      this.form.userBusinessVoList = data;
    },
    clear(type) {
      if (type == "user") {
        this.form.userId = "";
      } else {
        this.form.userBusinessVoList = [];
      }
    },
    getList() {
      this.$axios.post(
        "/web/measuring/api/costJurisdiction/list",
        {
          data: {
            customerCode: this.$store.state.user.customerCode,
            userId: this.userId,
          },
          pagination:true,
          current:this.pagination.currentPage,
          size:this.pagination.pageSize
        },
        (res) => {
          if (res.isSuccess) {
            this.tableData = res.data.dataList
            this.pagination.total = res.data.total
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    handlePageSize(pageSize) {
      this.pagination.pageSize = pageSize;
      this.getList();
    },
    pageChange(page) {
      this.pagination.currentPage = page;
      this.getList();
    },
    getCompany() {
      this.$axios.post(
        "/web/user/api/company/children",
        {
          data: {
            customerCode: this.$store.state.user.customerCode,
            flag: 3,
            id: this.$store.state.user.companyId,
          },
        },
        (res) => {
          if (res.isSuccess) {
            // this.dutyList = res.data;
            // this.filterDutyList = res.data
            const arr = this.filterPerson(
              JSON.parse(JSON.stringify(res.data)),
              "5"
            );
            this.userList = res.data;
            this.departMentList = this.filterPersonDepart(JSON.parse(JSON.stringify(res.data)),
              "2")
            this.companyList = arr;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    //过滤掉人
    filterPerson(treeData, excludeType) {
      // 辅助函数，用于递归检查并过滤节点
      function filterNode(node) {
        // 如果当前节点的type是要排除的，返回null
        if (node.type === excludeType) {
          return null;
        }

        // 递归处理子节点
        if (node.childrenList) {
          node.childrenList = node.childrenList.filter(filterNode);
          // 如果子节点数组为空，则删除该属性以避免不必要的嵌套
          if (node.childrenList.length === 0) {
            delete node.childrenList;
          }
        }

        // 返回当前节点（如果它没有被排除）
        return node;
      }

      // 调用辅助函数并返回过滤后的树
      return treeData.filter(filterNode);
    },
    filterPersonDepart(treeData, excludeType){
      // 辅助函数，用于递归检查并过滤节点
      function filterNode(node) {
        // 如果当前节点的type是要排除的，返回null
        if (node.type === excludeType) {
          node.childrenList=[]
          return node;
        }

        // 递归处理子节点
        if (node.childrenList) {
          node.childrenList.map(item=>{
            item.fullName=item.type=='2'?((item.fullName||node.name)+'/'+item.name):''
          })
          node.childrenList = node.childrenList.filter(filterNode);
          // 如果子节点数组为空，则删除该属性以避免不必要的嵌套
          if (node.childrenList.length === 0) {
            delete node.childrenList;
          }
        }

        // 返回当前节点（如果它没有被排除）
        return node;
      }

      // 调用辅助函数并返回过滤后的树
      return treeData.filter(filterNode);
    },
    dictSelect(data) {
      console.log(data, "+++");
    },
  },
};
</script>
<style lang="scss" scoped>
.permissionSetting {
  background: #fff;
  .top{
    padding: 10px 10px;
    display: flex;
  }
}
</style>
