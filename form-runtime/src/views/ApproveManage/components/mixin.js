import {deepClone} from '@/utils'
const mixin = {
  methods:{
    generateTableData(data){
      this.pagination.total = data.length
      var current = this.pagination.pages
      var size = this.pagination.size
      var start = (current - 1)*size
      var end = start + size
      return data.slice(start,end)
    },
    pageChange(current){
      this.pagination.pages = current
      this.tableData = this.generateTableData(this.wholeData)
    },
    sizeChange(size){
      // console.log('sizeChange')
      // this.$set('this.pagination','pages',1)
      this.pagination.pages = 1
      this.pagination.size = size
      this.tableData = this.generateTableData(this.wholeData)
    },
    queryData(){
      var data = deepClone(this.originData)
      //发起人
      if(this.initiator){
        data = data.filter(item=>{
          return item.initiator.indexOf(this.initiator)>-1
        })
      }
      //流程名称
      if(this.flowName){
        data = data.filter(item=>{
          let formName = item.name || item.formName
          return formName.indexOf(this.flowName) > -1
        })
      }

      //流程状态
      if(this.status){
        data = data.filter(item=>{
          let flowStatus = item.flowStatus || item.status
          return this.status == flowStatus
        })
      }

      //发起时间
      if(this.startDate && this.endDate){
        data = data.filter(item=>{
          let createTime = item.initiatorDate || item.createDate
          let createTimeUnix = new Date(createTime).getTime()
          let startUnix = new Date(this.startDate).getTime()
          let endUnix = new Date(this.endDate).getTime()
          return (createTimeUnix<=endUnix && createTimeUnix>=startUnix)
        })
      }
      this.wholeData = deepClone(data)
      this.tableData  = this.generateTableData(data)
    },
  }
}
export default mixin
