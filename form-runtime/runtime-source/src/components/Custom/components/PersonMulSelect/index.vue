<!--
 * @Descripttion: 人员多选
 * @Author: zhengzetao
 * @Date: 2024-12-4
-->

<template>
  <div class="custom-container">

    <MySendFlowList :visible.sync="flowListVisible" v-model="currentInfoObj" @selectFlow="selectFlow">
      <template scope="scope">
        <div v-if="printRead" class="print-read-label">
          <span v-for="(item,index) in dataShowList" :key="index">{{ item.name }}<i v-if="(index+1) != dataShowList.length">、</i></span>
        </div>
        <template v-else>
          <div v-if="dataShowList.length">
            <div class="personSelect-wrap">
              <div class="name-wrap">
                <draggable v-model="dataShowList" @end="onDragEnd(dataShowList)">
                  <span class="name" v-for="(item,index) in dataShowList" :key="index">{{ item.name }}<i v-if="(index+1) != dataShowList.length">、</i></span>
                </draggable>
              </div>
              <div class="person-icon-wrap" v-show="!disabled">
                <i class="el-icon-user-solid person-icon1" @click="openInfoDialog"></i>
                <i class="el-icon-user person-icon2" @click="openInfoDialog"></i>
              </div>
            </div>
          </div>
          <el-button v-else type="primary" :disabled="disabled" @click="openInfoDialog" style="margin-left:5px;">{{ placeholder }}</el-button>
        </template>
      </template>
    </MySendFlowList>
  </div>
</template>
<script>
import Api from '@/api';
import MySendFlowList from './MySendFlowList.vue';
import draggable from 'vuedraggable';
import { parseJsonArray, parseJsonObject } from '@/utils/parse-value';
export default {
  name: 'PersonMulSelect',
  components: {
    MySendFlowList,
    draggable
  },
  props: {
    value: {
      type: String,
      default: ''
    },
    placeholder: {
      type: String,
      default: ''
    },
    disabled: { // value
      type: [Boolean], // [Array, String, Number]
      default() {
        return false;
      }
    },
    printRead: { // value
      type: [Boolean], // [Array, String, Number]
      default() {
        return true;
        // return false;
      }
    }
  },
  data() {
    return {
      currentInfoObj: this.value,
      flowListVisible: false,
      dataShowList:[]
    };
  },
  created() {
  },
  mounted() {
    console.log('this',this)
    this.dataShowList = parseJsonArray(parseJsonObject(this.currentInfoObj).flowList);
   },
  watch: {
    value(val) {
      console.log('val-人员多选-传值',val)
      this.currentInfoObj = val;
      this.dataShowList = parseJsonArray(parseJsonObject(val).flowList);
    },
    placeholder(val) {
      // console.log('======placeholder======',val)
    },
    currentInfoObj(val){
      console.log('emit-人员多选-控件赋值',val)
      const flowList = parseJsonArray(parseJsonObject(val).flowList)
      let arr = flowList.map(x=>x.id)
      console.log('arr--currentInfoObj', arr)
      this.$parent.$set(this.$parent.dataModels,this.$parent.modelName+'__formPersonId',arr) // 指定表单人员虚拟字段（表单指定人员要的是id）

      this.$emit('input', val)
    },
    disabled(val){
      console.log('====disabled===',this.disabled)
    },
    printRead(val){
      console.log('====printRead22===',this.printRead)
    },
  },
  computed: {
  },
  methods: {
    onDragEnd(personList) {
      this.selectFlow(JSON.stringify({
        flowList: personList
      }));
    },
    openInfoDialog(data){
      this.flowListVisible = true;
    },
    selectFlow(data) { // 选择赋值
      console.log('selectFlow-data',data)
      this.currentInfoObj = data;
      this.dataShowList = parseJsonArray(parseJsonObject(data).flowList);
      // 去重
      // const newArr = JSON.parse(data).flowList.reduce((item, next) => {
      //   obj[next.name] ? '' : obj[next.name] = true && item.push(next);
      //   return item;
      // }, []);
      console.log('this.dataShowList',this.dataShowList)
      this.flowListVisible = false;

      // 添加表单校验
      this.$nextTick(() => { 
        this.$parent.$parent.validate()
      })
    },
  },
};
</script>
<style lang="scss" scoped>
.custom-container {
  width: 100%;
}

.personSelect-wrap{
  display:flex;
  padding: 4px;
  .name-wrap {
    flex:1;
    .name {
      white-space: nowrap;
    }
  }
  .person-icon-wrap {
    font-size: 20px;
    width: 40px;
    position: relative;
    cursor: pointer;
    color: #409EFF;
    .person-icon1{
      position: absolute;
      top: 36%;
      right: 11px;
    }
    .person-icon2 {
      position: absolute;
      top: 36%;
      right: 3px;
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
