<!--
 * @Author: oygsky
 * @Date: 2025-12-15 16:05:56
 * @LastEditors: oygdeMac-mini.local
 * @LastEditTime: 2025-12-16 08:55:10
 * @Description: 唧唧复唧唧
 * @FilePath: /rsh-cloud-invest-power-system/src/views/GroupApproveManage/components/FormCommonFlowPost/index copy.vue
-->
<!--
 * @Descripttion: 公共流程关联组件（选择多个数据）
 * @Author: zhengzetao
 * @Date: 2025-1-21
-->

<template>
  <div class="custom-container">
    <!-- <el-button type="primary" icon="el-icon-edit" @click="openInfoDialog"></el-button> -->
    <div class="btn-title" style="font-size:20px;width: 128px;">
      <i class="el-icon-suitcase-1" style="color:rgb(251,191,36);"></i>
      <el-link :underline="false" @click="openInfoDialog" :disabled="noClick" style="font-size:16px;margin-left: 4px;">关联流程({{ dataShowList.length }})</el-link>
    </div>
    <ul v-if="dataShowList.length" class="flowList-ul">
      <li class="flowList-li" v-for="(item,index) in dataShowList" :key="index">
        <el-link @click="checkDetail(item)">
          <i class="el-icon-s-claim" style="color: rgb(55, 126, 240);"></i>
          <span>{{ item.name }}</span>
        </el-link>
        <i v-if="!noClick" class="el-icon-close upload-close" @click.stop="handleRemove(item['id'])"></i>
        <!-- <i v-if="!noClick" class="el-icon-close upload-close" @click.stop="handleRemove(item['key'])"></i> -->
      </li>
    </ul>
    <MySendFlowList :visible.sync="flowListVisible" v-model="currentInfoObj" @selectFlow="selectFlow" :fieldSelectType="fieldSelectType">
    </MySendFlowList>

  </div>
</template>
<script>
import Api from '@/api';
import mixin from './mixin.js';
import MySendFlowList from './MySendFlowList.vue';
import { deepClone } from '@/utils/index';
import { localstorageGet } from '@/utils/auth';

export default {
  name: 'FormCommonFlowPost',
  components: {
    MySendFlowList
  },
  mixins:[mixin],
  props: {
  enableData: {
    type: [Array, Object],
    default: null
  },
  fromPage: {
    type: String,
    default: ''
  },
  isExamine: {
    type: Boolean,
    default: false
  },
  isReInitiate: {
    type: Boolean,
    default: false
  },
  btnVisible: {
    type: Boolean,
    default: false
  },
  value: {
    type: [String, Object],
    default: ''
  },
  placeholder: {
    type: String,
    default: ''
  },
  disabled: {
    type: Boolean,
    default: false
  },
  printRead: {
    type: Boolean,
    default: false
  },
  flowInstanceBizRelevanceList: {
    type: Array,
    default: () => []
  },
  newFlowList: {
    type: Array,
    default: () => []
  },
  initiatorId: {
    type: String,
    default: () => localstorageGet('userId') || ''
  }
},
  data() {
    return {
      currentInfoObj: this.value,
      flowListVisible: false,
      fieldSelectType: 'flow',
      typeList: {
        'flow': {
          name:'流程',
        },
      },
      btnVisible:false,
      dataShowList:[],
      // 使用 prop 覆盖 mixin 中的 data
      initiatorId: this.initiatorId,
    };
  },
  created() {
  },
  mounted() {
    console.log('flowInstanceBizRelevanceList',this.flowInstanceBizRelevanceList)
    
    this.dataShowList = [];
    this.flowInstanceBizRelevanceList && this.flowInstanceBizRelevanceList.forEach(async x=>{
      if (x.otherBiz == 'commonFlow') { // commonFlow是关联流程的标识
        let result = await this.getDetailByFlowInstanceId(x.otherBizId);
        console.log('result',result)
        this.dataShowList.push({
          name: result.name,
          id: x.otherBizId,
          rowData: JSON.stringify(result)
        })
      }
    })
    console.log('this.dataShowList',this.dataShowList)
    console.log('this.newFlowList',this.newFlowList)
   },
  watch: {
    value(val) {
      console.log('val-流程多选-传值',val)
      this.currentInfoObj = val;
      if (JSON.parse(val)?.flowList){
        this.dataShowList = JSON.parse(val).flowList;
      }
    },
    placeholder(val) {
      // console.log('======placeholder======',val)
    },
    currentInfoObj(val){
      // console.log('emit-流程多选-控件赋值',val)
    },
    disabled(val){
      console.log('====disabled===',this.disabled)
    },
    printRead(val){
      console.log('====printRead22===',this.printRead)
    },
    newFlowList(val){
      // console.log('====newFlowList===',val)
      if (val.length) {
        this.dataShowList = deepClone(val);
      }
    },
  },
  computed: {
    fieldType(){
      let copyType = JSON.parse(JSON.stringify(this.fieldSelectType))
      var str = '';
      for (var item in this.typeList){
        var result = new RegExp(item,'i').test(copyType);
        if (result) {
          str = item;
          break;
        }
      }
      return str
    },
    noClick() {
      if (this.fromPage == 'startFlow') {
        return false;
      } else {
        if(this.isExamine === false && this.isReInitiate === false){ // 查看流程
          return true
        } else {
          if(this.isExamine === false){
            return false
          }
        }
        if (this.fromPage == 'finishedView' || this.fromPage == 'submitView' || this.fromPage == 'submittedView') {
          return true;
        }
        if ((this.enableData && this.enableData.findIndex(item => item == 'fileupload_common_file') == -1) || this.btnVisible === false) {
          if (localstorageGet('userId') == this.initiatorId) {
            return false;
          } else {
            return true;
          }
        }
        return false;
      }
    },
  },
  methods: {
    // 根据流程实例Id获取流程详情
    getDetailByFlowInstanceId(flowInstanceId){
      return new Promise((resolve,reject)=>{
        const parmas = {
          id: flowInstanceId,
          initiator:"all" // 审批人查不到发起人的流程实例详情, 传这个参数可以查到发起人的数据，不做发起人限制
        };
        this.$axios.post(
          Api.schedule.getFlowInstanceList,
          {
            data:parmas,
            pagination: false,
          },
          res => {
            if (res.isSuccess) {
              resolve(res.data[0])
            } else {
              this.$message.error(res.message);
            }
          }
        );
      })
    },
    handleRemove(k, formType) {
      console.log('k',k)
      console.log('this.dataShowList',this.dataShowList)
      console.log(111,this.dataShowList.findIndex(item => item.id == k))
      this.dataShowList.splice(this.dataShowList.findIndex(item => item.id == k), 1);
    },
    checkDetail(item){
      console.log('checkDetail',item)
      if (item.rowData) {
        this.previewHandle(JSON.parse(item.rowData), false)
      }
    },
    openInfoDialog(data){
      this.flowListVisible = true;
    },
    selectFlow(data) { // 选择赋值
      console.log('selectFlow-data',data)
      this.dataShowList = this.dataShowList.concat(data.flowList);
      this.dataShowList = this.dataShowList.filter((value, index, self) =>
        index === self.findIndex((t) => t.id === value.id)
      );

      // this.dataShowList = data.flowList;
      console.log('this.dataShowList',this.dataShowList)
      this.flowListVisible = false;
    },
  },
};
</script>
<style lang="scss" scoped>
.custom-container {
  width: 100%;
  display: flex;
  align-items: center;
}

.btn-title {
  width: 95px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
}

.flowList-ul {
  display: flex;
  margin-left: 5px;
  flex-wrap: wrap;
  .flowList-li {
    margin-right: 5px;
    margin-top: 5px;
    background: rgb(230, 238, 247);
    padding: 1px 5px;
    display: flex;
    align-items: center;
    
    i.el-icon-close {
      margin-left: 5px;
      color: rgb(55, 126, 240);
    }
    i.el-icon-close:hover {
      background: #377ef0;
      color: #fff;
    }
  }
}

table {
  width: 100%;
}

td {
  text-align: center;
  padding: 0 2px;
}

</style>