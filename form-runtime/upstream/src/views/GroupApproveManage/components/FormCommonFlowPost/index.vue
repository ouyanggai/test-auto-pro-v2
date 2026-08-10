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
      <el-link :underline="false" @click="openInfoDialog" :disabled="noClick" style="font-size:16px;margin-left: 4px;">关联流程({{ totalCount || dataShowList.length }})</el-link>
    </div>
    <ul v-if="dataShowList.length" class="flowList-ul">
      <li class="flowList-li" v-for="(item,index) in displayList" :key="index">
        <el-link @click="checkDetail(item)">
          <i class="el-icon-s-claim" style="color: rgb(55, 126, 240);"></i>
          <span>{{ item.name }}</span>
        </el-link>
        <i v-if="!noClick" class="el-icon-close upload-close" @click.stop="handleRemove(item['id'])"></i>
        <!-- <i v-if="!noClick" class="el-icon-close upload-close" @click.stop="handleRemove(item['key'])"></i> -->
      </li>
      <li v-if="showExpandBtn" class="flowList-li flowList-expand" @click="loadMore" style="cursor:pointer;">
        <span>{{ expandBtnText }}</span>
      </li>
      <li v-if="loading" class="flowList-li" style="cursor:default;">
        <i class="el-icon-loading" style="color: rgb(55, 126, 240);"></i>
        <span style="margin-left:4px;">加载中...</span>
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
  props: ['enableData', 'fromPage','isExamine','isReInitiate','btnVisible','value','placeholder','disabled','printRead','flowInstanceBizRelevanceList','newFlowList','initiatorId'],
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
      pendingFlowItems: [],   // 尚未请求的流程项
      totalCount: 0,           // 过滤后的总条数
      showAll: false,
      displayLimit: 10,
      loading: false,
    };
  },
  created() {
  },
  async mounted() {
    console.log('flowInstanceBizRelevanceList',this.flowInstanceBizRelevanceList)

    this.dataShowList = [];

    // 先筛选出关联流程项
    const flowItems = (this.flowInstanceBizRelevanceList || [])
      .filter(x => x.otherBiz === 'commonFlow');

    if (flowItems.length === 0) return;

    this.totalCount = flowItems.length;

    // 首次只取前 10 条，剩余的点展开再拉取
    const INITIAL = 10;
    const firstBatch = flowItems.slice(0, INITIAL);
    this.pendingFlowItems = flowItems.slice(INITIAL);

    this.loading = true;
    this.dataShowList = await this.fetchBatch(firstBatch);
    this.loading = false;

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
    // 展示列表：默认只展示前 displayLimit 条，点展开后显示全部
    displayList() {
      if (this.showAll) return this.dataShowList;
      return this.dataShowList.slice(0, this.displayLimit);
    },
    // 还有未拉取的数据，或已拉取完毕但超过显示上限
    showExpandBtn() {
      return this.pendingFlowItems.length > 0 || this.dataShowList.length > this.displayLimit;
    },
    // 展开按钮文案（显示未展示的数量）
    expandBtnText() {
      if (this.showAll) return '收起';
      // 未拉取的 + 已拉取但折叠隐藏的
      const hidden = this.pendingFlowItems.length + Math.max(0, this.dataShowList.length - this.displayLimit);
      return '展开剩余(' + hidden + ')';
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
              reject(new Error(res.message));
            }
          }
        );
      })
    },
    // 批量拉取流程详情（并发控制）
    async fetchBatch(items) {
      const CONCURRENCY = 6;
      const results = [];

      for (let i = 0; i < items.length; i += CONCURRENCY) {
        const batch = items.slice(i, i + CONCURRENCY);
        const batchResults = await Promise.all(
          batch.map(x =>
            this.getDetailByFlowInstanceId(x.otherBizId)
              .then(data => ({ success: true, data, id: x.otherBizId }))
              .catch(() => ({ success: false, id: x.otherBizId }))
          )
        );

        batchResults.forEach(item => {
          if (item.success && item.data) {
            results.push({
              name: item.data.name,
              id: item.id,
              rowData: JSON.stringify(item.data)
            });
          }
        });
      }

      return results;
    },
    // 展开/加载更多（每批完成后即时展示）
    async loadMore() {
      if (this.pendingFlowItems.length === 0) {
        // 已全部拉取，仅切换展开/收起
        this.showAll = !this.showAll;
        return;
      }

      const items = this.pendingFlowItems;
      this.pendingFlowItems = [];
      this.showAll = true;
      this.loading = true;

      const CONCURRENCY = 6;
      for (let i = 0; i < items.length; i += CONCURRENCY) {
        const batch = items.slice(i, i + CONCURRENCY);
        const batchResults = await Promise.all(
          batch.map(x =>
            this.getDetailByFlowInstanceId(x.otherBizId)
              .then(data => ({ success: true, data, id: x.otherBizId }))
              .catch(() => ({ success: false, id: x.otherBizId }))
          )
        );

        const newItems = [];
        batchResults.forEach(item => {
          if (item.success && item.data) {
            newItems.push({
              name: item.data.name,
              id: item.id,
              rowData: JSON.stringify(item.data)
            });
          }
        });

        // 每批请求完立即追加，不等剩余批次
        this.dataShowList = this.dataShowList.concat(newItems);
      }

      this.loading = false;
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
