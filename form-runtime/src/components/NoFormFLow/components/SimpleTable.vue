<template>
  <el-form ref="ruleForm" :model="{tableData}" class="table-form">
    <el-table
      ref="multipleTable"
      :data="tableData"
      :border="tableConfig.border"
      style="width: 100%"
      :header-align="tableConfig.align"
      @selection-change="handleSelectionChange"
      >
      <el-table-column
        type="selection"
        width="55"
        v-if="tableConfig.isSelection"
        >
      </el-table-column>
      <el-table-column
        type="index"
        v-if="tableConfig.isIndex"
      >
      </el-table-column>
      <el-table-column
        v-if="tableConfig.isRadio"
        label="选择"
        width="50"
      >
      <template slot-scope="scope" >
        <el-radio v-model="radio" :label="scope.row.id"><i></i></el-radio>
      </template>
      </el-table-column>
      <el-table-column v-for="col in tableConfig.column" :key="col.prop"
                      :prop="col.prop"
                      :label="col.label"
                      :width="col.width"
                      :align="col.align || 'center'"
                      >
          <template slot-scope="scope" >
            <div v-if="col.slot">
              <!-- <el-form-item :prop="'tableConfig.' + scope.$index +'.'+ col.prop" :rules="col?.slot?.rules || "> -->
              <el-form-item :prop="'tableData.' + scope.$index +'.'+ col.prop" :rules="col.slot.rules || []">
              <el-input
                v-if="col.slot.type == 'input'"
                v-model="scope.row[col.prop]"
                :disabled="col.slot.disabled"
                @input="(value)=>inputEvent(value,col,scope.$index)"
                :maxlength="col.slot.maxlength || '200'"
                placeholder=""/>
              <el-input
                v-if="col.slot.type == 'text'"
                type="textarea"
                :row="col.slot.row || 2"
                v-model="scope.row[col.prop]"
                :disabled="col.slot.disabled"
                placeholder=""/>
              <el-input
                v-if="col.slot.type == 'number'"
                type="number"
                v-model="scope.row[col.prop]"
                :controls="false"
                :disabled="col.slot.disabled"
                :maxlength="col.slot.maxlength || '200'"
                placeholder=""/>
              <el-date-picker
                v-if="col.slot.type == 'date'"
                v-model="scope.row[col.prop]"
                format="yyyy-MM-dd"
                value-format="yyyy-MM-dd"
                type="date"
                :disabled="col.slot.disabled"
                placeholder=""/>
              <el-select v-if="col.slot.type == 'select'" v-model="scope.row[col.prop]" placeholder="请选择合同"
                          @change="(val)=>{col.slot.changeEvent(val,scope.$index)}"
                          :disabled="col.slot.disabled"
                          >
                  <el-option
                    v-for="item in col.slot.options"
                    :key="item.value"
                    :label="item.label"
                    :value="item.value">
                  </el-option>
                </el-select>
              </el-form-item>
            </div>
            <div v-else slot-scope="scope">
              {{ scope.row[col.prop] }}
            </div>
          </template>
          <el-table-column v-if="col.children && col.children.length" v-for="val in col.children"
                          :key="val.prop"
                          :prop="val.prop"
                          :label="val.label"
                          :width="val.width"
                          :align="col.align || 'center'"
                          >
                          <template slot-scope="scope" >
                            <div v-if="val.slot">
                              <el-input
                                v-if="val.slot.type == 'input'"
                                v-model="scope.row[val.prop]"
                                :disabled="val.slot.disabled"
                                @input="(value)=>inputEvent(value,val,scope.$index)"
                                :maxlength="val.slot.maxlength || '200'"
                                placeholder=""/>
                              <el-input
                                v-if="val.slot.type == 'text'"
                                type="textarea"
                                :row="val.slot.row || 2"
                                v-model="scope.row[val.prop]"
                                :disabled="val.slot.disabled"
                                placeholder=""/>
                              <el-input
                                v-if="val.slot.type == 'number'"
                                type="number"
                                v-model="scope.row[val.prop]"
                                :controls="false"
                                :disabled="val.slot.disabled"
                                :maxlength="val.slot.maxlength || '200'"
                                placeholder=""/>
                              <el-date-picker
                                v-if="val.slot.type == 'date'"
                                v-model="scope.row[val.prop]"
                                type="date"
                                format="yyyy-MM-dd"
                                value-format="yyyy-MM-dd"
                                :disabled="val.slot.disabled"
                                placeholder=""/>
                                <el-select v-if="val.slot.type == 'select'" v-model="scope.row[col.prop]" placeholder="请选择合同"
                                  @change="(val)=>{val.slot.changeEvent(val,scope.$index)}"
                                  :disabled="val.slot.disabled"
                                  >
                                <el-option
                                  v-for="item in val.slot.options"
                                  :key="item.value"
                                  :label="item.label"
                                  :value="item.value">
                                </el-option>
                              </el-select>
                            </div>
                            <div v-else slot-scope="scope">
                              {{ scope.row[val.prop] }}
                            </div>
                          </template>
          </el-table-column>
      </el-table-column>
      <el-table-column
        label="操作"
        v-if="tableConfig.action"
        :width="tableConfig.action.width">
        <template slot-scope="scope">
          <!-- <el-button @click="handleClick(scope.row)" type="text" size="small">查看</el-button> -->
          <!-- <el-button type="text" v-for="val in tableConfig.action.buttons" @click="val.clickEvent">{{ val.label }}</el-button> -->
          <el-button type="text" v-for="val in tableConfig.action.buttons" @click="actionClick(val,scope.row,scope.$index)">{{ val.label }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-form>
</template>

<script>
import {deepClone} from '@/utils'
export default {
  name:'SimpleTable',
  components: {},
  props: ['tableConfig','tableData','multipleSelection'],
  data() {
    return {
      radio:'',
    };
  },
  created() {},
  mounted() {
    if(this.tableConfig.isSelection){
      if (this.multipleSelection.length) {
        var multipleSelection = deepClone(this.multipleSelection)
        multipleSelection.forEach(row => {
          var id = row.id
          var find = this.tableData.find(el=>el.id == id)
          if(find){
            this.$nextTick(()=>{
              this.$refs.multipleTable.toggleRowSelection(find,true);
            })
          }
        });
      }
    }
  },
  watch: {},
  computed: {},
  methods: {
    actionClick(item,row,index){
      if(item.clickEvent&& typeof(item.clickEvent) == 'function' ){
        item.clickEvent({row,index})
      }
    },
    inputEvent(value,item,index){
      if(item.slot.inputEvent && typeof(item.slot.inputEvent) == 'function'){
        item.slot.inputEvent(value,index)
      }
    },
    handleSelectionChange(val) {
      this.$emit('update:multipleSelection',val)
    },
    async validateData(){
      var ref = true
      await this.$refs.ruleForm.validate(function(res){
        ref = res
      })
      if(ref){
        return true
      }else{
        return false
      }
    }
  },
};
</script>
<style lang="scss" scoped>
::v-deep .el-input__inner,::v-deep .el-textarea__inner{
  padding: 0 4px !important;
}
::v-deep .el-input-number.el-input-number--mini,.el-date-editor.el-input--mini {
  width: 100%;
}
::v-deep .el-input__prefix{
  display: none !important;
}
::v-deep .cell{
  overflow: visible;
}
::v-deep .el-table__body-wrapper{
  overflow: visible !important;
}
::v-deep .el-table{
  overflow: visible !important;
}
.table-form .el-form-item--mini.el-form-item{
  margin-bottom: 0;
}
::v-deep .el-input.is-disabled .el-input__inner{
  color: #555;
  // background-color:#fff;
}
::v-deep .el-input-number{
  text-align: left;
}
/* 谷歌 */
::v-deep input::-webkit-outer-spin-button,
::v-deep input::-webkit-inner-spin-button {
    -webkit-appearance: none;
    appearance: none;
    margin: 0;
}
</style>
