//无表单的几种状态 1 create 新建； 2 edit 编辑 开放全部编辑权限； 3 examine 审核 按照流程配置开放编辑； 4 preview 查看

import Api from '@/api';
import {localstorageGet} from '@/utils/auth'
var apiList = {
  'buy_demand':{save:Api.noForm.saveBuyDemand,update:Api.noForm.updateBuyDemand},
  'buy_order':{save:Api.noForm.savebuyOrder,update:Api.noForm.updatebuyOrder},
  'buy_plan':{save:Api.noForm.createBuyPlan,update:Api.noForm.createBuyPlan},
  'expense_reimbursement':{},
  'contract_pay_request':{save:Api.noForm.contractPaymentSave,update:Api.noForm.contractPaymentSave},
  'contract_review':{save:Api.noForm.saveContractReview,update:Api.noForm.saveContractReview},
  'invoice':{save:Api.noForm.saveInvoice,update:Api.noForm.updateInvoice},
}
const mixin = {
  mounted() {
    this.$bus.$off('saveBiz')
    this.$bus.$on('saveBiz',()=>{
      this.saveBiz()
    })
  },
  methods:{
    async saveBiz(){
      // this.$bus.$emit('saveSucess')
      // return
      // var componentName = this.$options.name
      // if(['buy_order'].indexOf(componentName) > -1 ){
      //   this.$bus.$emit('saveSucess')
      // }else{
      var mainData = await this.processSaveData()
      if(!mainData)return false
      // console.log('mainData',mainData)
      // return
      this.saveData({data:mainData},'update').then(res=>{
        // return
        if(res.isSuccess){
          this.bizId = mainData.id
          this.$bus.$emit('saveSucess')
        }else{
          this.$message.error(res.message)
        }
      })
      // }
    },
    submit(status){
      var flow = this.$refs.flow
      flow.checkFlowPermission().then(async res=>{
        if(res){
          var componentName = this.$options.name
          var isDraft = true
          status == 1 ? isDraft = false : isDraft = true
          if(this.operaType == 'create'){
            if(this.bizId){
              //直接提交流程，不调用业务接口
              // console.log('create---->')
              this.postFlow(isDraft,this.bizId,componentName)
              return
            }
          }
          var mainData = await this.processData()
          if(!mainData)return false
          mainData.status = status
          mainData.projectId = localstorageGet('projectId') || ''
          if(!mainData.projectId){
            mainData.status = 0
            isDraft = true
          }
          // console.log(mainData)
          // return
          //----------提交业务----------
          this.saveData({data:mainData},'save').then(res=>{
            if(res.isSuccess){
              this.bizId = res.data.id
              //----------如果有文件，提交文件
              if(mainData.attachment)this.saveBatchFile(mainData.attachment.split(','),this.bizId)
              //----------提交流程----------
              this.postFlow(isDraft,this.bizId,componentName)
              if(!mainData.projectId){
                this.$message.warning('部分数据缺失，该条流程已转成草稿，您可以重新发起')
              }
            }else{
              this.$message.error(res.message)
            }
          })
        }else{
          this.$message.error('无权限发起流程')
        }
      })
    },
    saveData(data,type){
      var componentName = this.$options.name
      return this.$axios.post(apiList[componentName][type],data)
    },
    async reSubmit(type){
      var status,isDraft
      if(type == 'submit'){
        isDraft = false
        status = 1
      }else{
        isDraft = true
        status = 0
      }
      var mainData = await this.processSaveData()
      if(!mainData)return false
      // console.log('mainData',mainData)
      // return
      var componentName = this.$options.name
      //----------更新业务 update----------
      this.saveData({data:mainData},'update').then(res=>{
        if(res.isSuccess){
          this.bizId = mainData.id
          //----------如果有文件，提交文件
          if(mainData.attachment)this.saveBatchFile(mainData.attachment.split(','),this.bizId)
          //----------提交流程----------
          var flow = this.$refs.flow
          if(!isDraft)flow.reSubmit(isDraft,true,this.bizId,componentName)
          else{
            this.$message.success('操作成功')
            this.$bus.$emit('success')
          }
        }else{
          this.$message.error(res.message)
        }
      })
    },
    postFlow(isDraft,bizId,componentName){
      var flow = this.$refs.flow
      flow.beforeHandle(isDraft,true,bizId,componentName)
    },
    //批量保存文件
    saveBatchFile(fileIds,relationId){
      const data = {
        relationId,
        fileIds
      };
      this.$axios.post(
        Api.budgetManage.saveBatchFile,
        { data }
      );
    },
    //批量获取文件
    getBatchFile(id){
      return this.$axios.post(
        Api.schedule.getAttachmentList,
        {
          data: {
            relationId: id
          },
          fileType: 'ordinaryFile'
        })
    },
    //获取输入的权限
    getInputPermision() {
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          Api.qualityManage.findApprovePermission,
          {
            data: {},
            nodeProxyId: this.flowNodeProxyId
          },
          (res) => {
            let enableList = []
            if (res.data && res.data.flowNodeFieldPowerTemplateList) {
              let tmpList = res.data.flowNodeFieldPowerTemplateList || []
              enableList = tmpList.map(item => {
                return item.formFieldTemplateEnglishName
              })
            }
            resolve(enableList)
          }
        );
      })
    },
    //生成form列表
    initFormList(val,form){
      if(form === undefined)form = {id:''}
      val.forEach(item=>{
        item.forEach(el=>{
          var type = el.type
          if(type != 'label' && type != 'table'){
            if(el.children && el.children.length){
              // console.log('form',form)
              form = this.initFormList(el.children,form)
            }else{
              var prop = el.prop
              if(prop!=''){
                form[prop] = el.value || ''
              }
            }
          }
        })
      })
      return form
    },
    assignValue(arr,prop,key,val){
      arr.forEach(item=>{
        item.forEach(el=>{
          if(el.children){
            var children = el.children
            this.assignValue(children,prop,key,val)
          }else{
            if(el?.prop == prop){
              el[key] = val
            }
          }
        })
      })
    },
    getCompanyPersonnelTree(companyId) { // 获取公司人员架构数据
      return new Promise((resolve,reject)=>{
        this.$axios.post(
          // 放开
          Api.noForm.getAllUsersOfGroup,
          {
            data: {
              companyId
            }
          },
          res => {
            if (res.isSuccess) {
              // var data = res.data[0]?.childrenList || []
              // console.log('data',data)
              console.log('res.data',res.data)
              var currentCompany = this.findByCompanyId(companyId,res.data)
              // console.log('company',company)
              // var currentCompany = data.find(item=>item.id == companyId)
              if(currentCompany){
                var childrenList = currentCompany.childrenList
                var list = []
                this.getLastLevelList(childrenList,list)
                resolve(list)
              }
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    findByCompanyId(companyId,list){
      var hasFind = false,company = null
      for(let i=0;list[i];i++){
        var id = list[i].id
        if(id == companyId ){
          company = list[i]
          hasFind = true
          break
        }
      }
      if(!hasFind){
        var index = 0,res = null
        while(index < list.length){
          res = this.findByCompanyId(companyId,list[index].childrenList)
          index++
          if(res)break
          else res = false
        }
        return res
      }else{
        return company
      }
    },
    getLastLevelList(childrenList,returns){
      for(let i=0;childrenList[i];i++){
        if(childrenList[i].childrenList && childrenList[i].childrenList.length){
          this.getLastLevelList(childrenList[i].childrenList,returns)
        }else{
          returns.push({
            name:childrenList[i].name,
            value:childrenList[i].id,
            label:childrenList[i].name,
            id:childrenList[i].id
          })
        }
      }
    }
  }
}
export default mixin
