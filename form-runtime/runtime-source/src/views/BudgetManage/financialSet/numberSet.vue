<template>
  <div class="container">
    <el-row class="header">
      <el-col :span="10" :offset="4"  style="text-align: left;">费用预算归属公司名称</el-col>
      <el-col :span="10" style="text-align: center;">序号</el-col>
    </el-row>
    <div class="content">
      <el-row v-for="(val, index) in companyOption" :key="index" style="margin-top: 5px;text-align: center;">
        <el-col :span="10" :offset="4" style="text-align: left;">{{ val.name }}</el-col>
        <el-col :span="10">
          <el-input style="width: 120px;text-align: center;" v-model.trim="val.number"></el-input>
        </el-col>
      </el-row>
    </div>
    <div class="footer" style="text-align: center;margin-top: 20px;">
      <span class="dialog-footer">
        <el-button type="primary" @click="confirm">保 存</el-button>
      </span>
    </div>
  </div>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';
export default {
  name: '',
  components: {},
  props: [],
  data() {
    return {
      companyOption: []
    };
  },
  created() {
    this.init()
  },
  mounted() { },
  watch: {},
  computed: {},
  methods: {
    init(){
      this.getCompanyNumber().then(()=>{
        this.getParentCompanyList()
      })
    },
    handleClose(){

    },
    confirm(){
      let data = []
      this.companyOption.forEach(item=>{
        // if(item.number !== ''){
          let temp = {
            number:item.number,
            companyId:item.companyId
          }
          if(item.id)temp.id = item.id
          data.push(temp)
        // }
      })
      this.$axios.post(Api.budgetManage.saveCompanyNumber,{data:{},dataList:data},res=>{
        if(res.isSuccess){
          this.$message.success('保存成功')
          this.init()
        }
      })
    },
    getCompanyNumber(){
      return new Promise((resolve,reject)=>{
        this.$axios.post(Api.budgetManage.getCompanyNumberList,{},res=>{
          if(res.isSuccess){
            this.companyNumberList = res.data?.dataList || []
            resolve()
          }
        })
      })
    },

    getParentCompanyList() { // 查询公司列表
      this.$axios.post(
        Api.frameworkInfo.getCompanyFrameworkData,
        {
          data: {
            id: localstorageGet('companyId'), // 当前用的公司id
            flag: 1
          }
        },
        res => {
          var arr = []
          var fn = (list) => {
            list.forEach(item => {
              if (item.type == 1) {
                arr.push({
                  companyId: item.id,
                  name: item.name,
                  type: item.type,
                })
                if (item.childrenList && item.childrenList.length) {
                  fn(item.childrenList)
                }
              }
            })
          }
          fn(res.data)
          arr.forEach(item=>{
            let companyId = item.companyId
            let find = this.companyNumberList.find(el=>el.companyId == companyId)
            item.number = ''
            if(find){
              item.number = find.number
              item.id = find.id
            }
          })
          this.companyOption = arr
        }
      );
    },
  },
};
</script>
<style lang="scss" scoped>
.container {
  height: 100%;
  padding: 14px;
  background: #fff;

  .content {
    margin-top: 5px;
    margin: auto;
    width: 700px;
    height: 70vh;
    overflow: auto;
  }
  .header{
    margin: auto;
      width: 700px;
      color: #409EFF;
      font-weight: 600;
      font-size: 15px;
      text-align: center;
    }
}
</style>
