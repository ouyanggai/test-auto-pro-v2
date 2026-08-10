import Api from '@/api';
import { localstorageGet} from '@/utils/auth';
//处理从formmaking emit出来的数据和事件
const mixin = {
  methods: {
    /**
     * 从formmaking内部emit出来得方法
     * @param obj
     * formmaking emit出来的数据，包含需要调用的方法，以及数据给回formmaking的哪个组件
     * obj：{
     *  fun //需要调用的方法的名称
     *  field //数据给回到哪个组件
     *  }
     */
    customeFunc(obj) {
      const { fieldArr, fun } = obj
      // this.$refs.generateForm.setData({ [field]: '1235' })
      if(fun && typeof(fun) === 'string' && this[fun] && typeof(this[fun]) === 'function'){
        this[fun](fieldArr)
      }
    },
    setSelectOptions(field,list){
      this.$refs.generateForm.setOptions([field], {
        remote: true
      })
      this.$refs.generateForm.setOptionData([field], list)
    },
    //获取公司列表:集团公司,子公司以及相关方公司
    getCompanyList(fieldArr){
      Promise.all([this.getSonCompany(),this.getRelatedPartyByUser()]).then(resp=>{
        //子公司列表
        let company = resp[0]?.data
        let list = [
          {
            id:company[0].id,
            name:company[0].name
          }
        ]
        company.childrenList && company.childrenList.forEach(item=>{
          let tmp = {
            id:item.id,
            name:item.name
          }
          list.push(tmp)
        })
        ///组装相关方数据
        let relativeCompany = resp[1]?.data || []
        relativeCompany.forEach(item=>{
          let id = item.id
          if(list.findIndex(el=>el.id == id) == -1){
            list.push({
              id,
              name:item.name
            })
          }
        })
        fieldArr.forEach(item=>{
          let field = item
          this.$refs.generateForm.setOptions([field], {
            props: {
              value: 'id', // 动态数据选项值配置
              label: 'name' // 动态数据选项标签配置
            },
          })
          this.setSelectOptions(field,list)
        })
      })
    },
    //获取子公司和集团
    getSonCompany(){
      let data = {
        flag:2,
        id:localstorageGet('companyId')
      }
      return this.$axios.post(Api.frameworkInfo.getCompanyFrameworkData,{data})
    },
    // 获取客户下相关方列表
    getRelatedPartyByUser(){
      return this.$axios.post(Api.contractManage.contractInfo.getRelatedPartyByUser,{data: {}})
    },
    //获取人员列表
    getPersonList(fieldArr){
      let data = {
        flag:3,
        id:localstorageGet('companyId')
      }
      let personList = []
      this.$axios.post(Api.frameworkInfo.getCompanyFrameworkData,{data}).then(res=>{
        if(res.isSuccess){
          let data = res.data || []
          var fun = (list)=>{
            list.forEach(item=>{
              if(item.type == 5){
                let id = item.id
                if(personList.findIndex(el=>el.id == id) == -1)personList.push(item)
              }
              if(item.childrenList && item.childrenList.length){
                fun(item.childrenList)
              }
            })
          }
          fun(data)
          fieldArr.forEach(item=>{
            let field = item
            this.$refs.generateForm.setOptions([field], {
              props: {
                value: 'id', // 动态数据选项值配置
                label: 'name' // 动态数据选项标签配置
              },
            })
            this.setSelectOptions(field,personList)
          })
        }
      })
    }
  }
}
export default mixin
