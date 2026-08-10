<!--
 * @Descripttion: 合同列表
 * @Author: zhengzetao
 * @Date: 2024-05-18
-->
<template>
  <div class="dialog-container">
    <el-dialog :visible="visible" center append-to-body @close='handleClose' width="80%">
      <div>
        <el-input style="width:240px;margin-right: 10px;" v-model.trim="serachName" clearable placeholder="查询合同名称">
        </el-input>
        <el-button type="primary" @click="getList">查询</el-button>
        <dy-table
          :fetchData="getList"
          :keys="colKey"
          :list="contractList"
          :isPagination="true"
          :pagination="pagination"
          @rowClick="isRowClick"
        ></dy-table>
      </div>
      <span slot="footer" class="dialog-footer">
        <template>
          <el-button type="primary" @click="handleClose">关 闭</el-button>
          <el-button type="primary" @click="confirm">确 定</el-button>
        </template>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import { localstorageSet } from '@/utils/auth';
import { getObjById } from "@/utils";
import DyTable from '@/components/DyTable';
import Api from '@/api';

export default {
  name: '',
  components: { DyTable },
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    // selectFlowType: {
    //   type: String,
    //   default: ''
    // }
  },
  data() {
    return {
      contractBodyList:[],
      contractList: [],
      serachName: '',
      pagination: {
        total: 0,
        pages: 1,
        current: 1,
        size: 10
      },
      colKey: {
        contractName: {
          label: '合同名称',
          showTooltip: true,
        },
        contractNumber: {
          label: '合同编号',
          showTooltip: true,
          width: '160px',
        },
        fileName: {
          label: '合同文件',
          showTooltip: true,
          handle: function (scope, createElement) {
            let name = scope.row.contractReviewLogVoList ? scope.row.contractReviewLogVoList[0]['contractName'] : scope.row.fileName;
            return createElement('span',name);
          }
        },
        newContractBody: {
          label: '合同主体',
          showTooltip: true,
          // width: '140px',
        },
        // contractBody: {
        //   label: '合同主体',
        //   showTooltip: true
        // },
      },
      rowData:{}
    };
  },
  computed: {},
  watch: {
  },
  created() {
    this.getContractBodyList();
  },
  mounted() {
  },
  methods: {
    // 获取合同主体字典列表
    getContractBodyList(){
      this.$axios.post(Api.admin.findByDictCode,{
        data:{
          dictCode:'contract_mainBody'
        },
        pagination:false,
      },res=>{
        if(res.isSuccess){
          this.contractBodyList = res.data.dataList;
        }
      })
    },
    // 处理合同主体数据转换
    dealBodyList(item){
      let bodyStr = '';
      // console.log('this.contractBodyList',this.contractBodyList)
      // console.log('item.contractBody',item.contractBody)
      if (item.contractBody){
        let contractBodyListCopy = JSON.parse(JSON.stringify(this.contractBodyList));
        let contractBodyArrByDou = item.contractBody.split(',');
        for(var i = 0;i<contractBodyArrByDou.length;i++) {
          let contractBodyArrByFen = contractBodyArrByDou[i].split(':');
          let firstBody = getObjById(contractBodyListCopy,contractBodyArrByFen[0],'dictDataVos','dictValue')
          if (firstBody){
            bodyStr+=` ${firstBody.dictLabel}:${contractBodyArrByFen[1]},`
          }
        }
      }
      
      let newBodyStr = bodyStr.slice(0,-1);
      if (newBodyStr == '') {
        this.$set(item,'newContractBody',item.contractBody) // 原来的合同主体（兼容原来合同主体不在数据字典里）；同时兼容相关方的主体
      } else {
        this.$set(item,'newContractBody',newBodyStr) // 新组装的合同主体
      }
    },
    getList() {
      const params = {
        data: {
          contractName: this.serachName,
          userId:this.$store.state.user.userId,
          companyId:this.$store.state.user.companyId,
          status:'1',
          examineStatus:'1',
          contractSubtableVo:{
            stampStatus:"-1", // 传-1才能不查出已通过的盖章评审表单，区别于合同列表
            stampExamineStatus:"-1", // 传-1才能不查出已通过的盖章评审表单，区别于合同列表
            // stampStatus:null,
            // stampExamineStatus:null,
          }
        },
        size: this.pagination.size,
        pages: this.pagination.pages,
        pagination:true

      };
      this.$axios.post(
        Api.contractManage.contractInfo.getContractList,
        params,
        (res) => {
          if (res.isSuccess) {
            this.contractList = res.data.dataList || [];
            this.pagination.total = res.data.total || 0;

            this.contractList.forEach(async x=>{

              // 处理合同主体数据转换
              this.dealBodyList(x);

              // 获取文件
              // console.log(1,x.contractReviewLogVoList)
              if (x.contractReviewLogVoList){
                await this.getFileByBizId(x.contractReviewLogVoList[0]['id']).then(y=>{
                  if (y.data && y.data.length >0) {
                    this.$set(x,'fileUrl',y.data[0].fileUrl)
                    this.$set(x,'fileName',y.data[0].originFileName)
                  } else {
                    x.fileUrl = null;
                  }
                })
              }
            })
            // console.log('this.contractList',this.contractList)
            
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    //根据业务id获取文件
    getFileByBizId(id) {
      return this.$axios.post(
        Api.schedule.getAttachmentList, {
        data: {
          relationId: id
        }
      })
    },
    isRowClick(row) {
      // console.log('isRowClick',row.row.fileName)
      this.rowData = row.row;
      // localstorageSet('contract-seal', {
      //   name: row.row.fileName,
      //   url: row.row.fileUrl
      // });
    },  
    confirm(){
      if (!Object.keys(this.rowData).length) {
        this.$message.warning('请选择一条合同数据！')
        return;
      }
      this.$emit('confirmContract',this.rowData)
      this.handleClose();
    },
    handleClose() {
      this.$emit('update:visible', false);
    }
  }
};
</script>

<style lang="scss" scoped></style>
<style lang="scss">
</style>
