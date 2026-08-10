<!-- form组件 -->
<template>
  <div class="container">
    <el-form ref="form" :model="form" class="form-div" :class="noBorder?'noBorder':''" :style="borderTops">
      <!-- <FormItem :config="config" :form="form"></FormItem> -->
      <el-row v-for="(colVal ,index) in config" :key="index" :class="noBorder?'noBorder':''" :style="{'margin-bottom':marginBottom}">
        <el-col v-for="val in colVal" :offset="val.offset || 0" :span="val.span" :style = "{'height':val.height||''}" :class="val.type == 'button' ? 'flex-center':''">
          <div class="label" v-if="val.type == 'label' && val.title" :class="[noBorder?'noBorder':'',val.align == 'left' ? 'alignLeft':val.align || '']"
              :style="{'border-right':val.borderRight ? '1px solid #999':'','color':color,'padding':'0 '+(val.padding || '8px')}">
            <span v-if="val.required" style="color: #FF6600;">*</span>
            {{ val.title }}
          </div>
          <template v-else>
            <div v-if="val.type == 'button'">
            <el-button :type="val.buttonType || 'primary'"  @click="()=>{clickEvent(val)}" :disabled="val.disabled">{{ val.title }}</el-button>
          </div>
          <div v-if="val.children && val.children.length">
            <FormItem :config="val.children" :noBorder="false" :form="form"
              @inputEvent="inputEvent"
              @changeEvent="changeEvent"
              @clickEvent="clickEvent"
              :pickerOptions="pickerOptions"
              ></FormItem>
          </div>
          <div v-else class="input-div">
            <el-form-item :prop="val.prop" :class="val.borderRight?'borderRight':''" :rules="val.rules || null" v-if="val.type != 'table'">
              <div v-if="val.type == 'input'" class="input-container">
                  <el-input :placeholder="val.placeholder|| '请输入'"  v-model="form[val.prop]"
                            :disabled="val.disabled"
                            :maxlength="val.maxlength || '200'"
                            @input="(value)=>inputEvent(value,val)"
                            ></el-input>
              </div>
              <div v-if="val.type == 'textarea'" class="input-container">
                  <el-input :placeholder="val.placeholder|| '请输入'"  v-model="form[val.prop]"
                            :disabled="val.disabled"
                            :maxlength="val.maxlength || '200'"
                            type="textarea"
                            @input="(value)=>inputEvent(value,val)"
                            autosize
                            style="vertical-align: middle;"
                            ></el-input>
              </div>
              <div v-if="val.type == 'inputNum'" class="input-container">
                  <el-input type="number" :placeholder="val.placeholder|| '请输入'"  v-model="form[val.prop]"
                            :disabled="val.disabled"
                            :maxlength="val.maxlength || '200'"
                            :controls="false"
                            @input="(value)=>inputEvent(value,val)"
                            ></el-input>
              </div>

              <div v-if="val.type == 'select'" class="input-container">
                <el-select v-model="form[val.prop]" :placeholder="val.placeholder || '请选择'"
                          :filterable="val.filterable || false" @change="(value)=>{changeEvent(value,val)}"
                          :disabled="val.disabled" style="width: 100%;">
                  <el-option
                    v-for="(item,k) in val.options"
                    :key="k"
                    :label="item.label"
                    :value="item.value">
                  </el-option>
                </el-select>
              </div>
              <div v-if="val.type == 'date'" class="input-container" :class="noBorder?'noBorder':''">
                <el-date-picker :placeholder="val.placeholder|| '选择日期'"
                                  v-model="form[val.prop]" :type="val.dateType" :disabled="val.disabled"
                                  :format="val.formate || 'yyyy-MM-dd HH:mm:ss'" :value-format="val.formate || 'yyyy-MM-dd HH:mm:ss'"
                                  :picker-options = "pickerOptions"
                                  >
                                </el-date-picker>
              </div>
              <div v-if="val.type == 'checkbox'" class="checkbox-div">
                <el-checkbox-group v-model="form[val.prop]" :disabled="val.disabled">
                  <el-checkbox v-for="item in val.options" :label="item.label" :key="item.label"></el-checkbox>
                </el-checkbox-group>
              </div>
              <div v-if="val.type == 'radio'" class="input-container" >
                <el-radio-group v-model="form[val.prop]" :disabled="val.disabled">
                  <el-radio v-for="item in val.options" :label="item.value" :key="item.label">{{ item.label }}</el-radio>
                </el-radio-group>
              </div>
            </el-form-item>
            <div v-else-if="val.type == 'table'" style="padding: 15px;">
                <SimpleTable :tableConfig="tableConfig" :tableData="tableData" ref="simpleTable"></SimpleTable>
            </div>
          </div>
          </template>
        </el-col>
      </el-row>
    </el-form>
  </div>
</template>

<script>
import eleupload from '@/components/EleUpload';
import FormItem from './FormItem.vue';
import SimpleTable from './SimpleTable.vue';
export default {
  name:'CommonForm',
  components: {eleupload,FormItem,SimpleTable},
  props: {
    formConfig:{
      type:Array,
      default:()=>{
        return []
      }
    },
    id:{
      type:String,
      default:''
    },
    noBorder:{
      type:Boolean,
      default:false
    },
    borderTop:{
      type:String,
      default:''
    },
    initForm:{
      type:Object,
      default(){
        return{}
      }
    },
    tableConfig:{
      type:Object,
      default(){
        return{}
      }
    },
    tableData:{
      type:Array,
      default(){
        return []
      }
    },
    pickerOptions:{
      type:Object,
      default(){
        return{}
      }
    },
    color:{
      type:String,
      default:'#000'
    },
    marginBottom:{
      type:String,
      default:'0'
    }
  },
  data() {
    return {
      config:'',
      form:this.initForm
    };
  },
  created() {
    this.form = this.initForm
    this.config = this.formConfig
  },
  mounted() {},
  watch: {
    formConfig:{
      handler(val){
        this.config = val
      },
      deep:true,
    },
    initForm:{
      handler(val){
        this.form = val
      },
      deep:true
    }
  },
  computed: {
    borderTops(){
      if(this.borderTop == 'none'){
        return {
          'border-top':'none'
        }
      }
    }
  },
  methods: {
    inputEvent(value,item){
      if(item.inputEvent && typeof(item.inputEvent) == 'function'){
        item.inputEvent(value)
      }
    },
    setValue(){
      console.log(this.form)
    },
    changeEvent(value,item){
      if(item.changeEvent&& typeof(item.changeEvent) == 'function' ){
        item.changeEvent(value)
      }
    },
    clickEvent(item){
      if(item.clickEvent&& typeof(item.clickEvent) == 'function' ){
        item.clickEvent()
      }
    },
    clear(){
      console.log('form')
      this.$refs.form.clearValidate()
    },
    async getData(){
      var ref = true
      await this.$refs.form.validate(function(res){
        ref = res
      })
      if(ref){
        if(this.id)this.form.id = this.id
        return this.form
      }else{
        return false
      }
    }
  },
};
</script>
<style lang="scss" scoped>
.container{
  width: 100%;
  .form-div{
    border: 1px double #999999;
    margin: 0 auto;
  }
  .form-div.noBorder{
    border:none;
  }
}
.el-row{
  display: flex;
  border-bottom: 1px solid #999999;
  .input-container{
    padding: 2px 8px;
    // border-right: 1px solid #999;
    line-height: 36px;
  }
  .input-container.noBorder{
    border-right: none;
  }
  .el-col:last-child .input-container{
    border: none;
  }
}
.el-row.noBorder{
  border-bottom: none;
}
.el-row:last-child{
  border: none;
}
.label{
  width: 100%;
  height: 100%;
  padding: 0 8px;
  box-sizing: border-box;
  border-right: 1px  solid #999999;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
}
.label.noBorder{
  border-right: none;
}
.checkbox-div{
  padding: 0 10px;
  display: flex;
  align-items: center;
}
::v-deep .el-form-item--mini.el-form-item{
  margin: 0;
}
::v-deep .el-date-editor.el-input, .el-date-editor.el-input__inner{
  width: 100% !important;
}
.borderRight{
  border-right:1px solid #999;
}
.alignLeft{
  justify-content: start;
}
.alignRight{
  justify-content: end;
}
.el-radio{
  margin: 2px 4px;
}
::v-deep .el-input.is-disabled .el-input__inner{
  color: #555;
  // background-color:#fff;
}
::v-deep .el-input-number .el-input__inner{
  text-align: left;
}
.flex-center{
  display: flex;
  align-items: center;
}
::v-deep input::-webkit-outer-spin-button,
::v-deep input::-webkit-inner-spin-button {
    -webkit-appearance: none;
    appearance: none;
    margin: 0;
}
.input-div{
  height: 100%;
  border-right: 1px solid #999;
}

::v-deep .el-col:last-child .input-div{
    border-right: none;
  }
</style>
