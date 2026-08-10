<template>
  <div class="budgetTypeSet">
    <el-tabs type="border-card" style="min-height: calc(100vh - 120px);">
      <el-tab-pane label="应用模板管理">
        <el-select v-model="value" placeholder="请选择" size="small" style="width: 360px;margin-right: 20px;">
          <el-option
            v-for="item in options"
            :key="item.value"
            :label="item.label"
            :value="item.value">
          </el-option>
        </el-select>
        <el-button type="primary" icon="el-icon-upload2" size="small" @click="openUploadDialogFunc">导入费用预算类型</el-button>
        <el-row style="margin-top: 20px;">
          <el-col :xs="24" :sm="24" :md="24" :lg="12" :xl="12">
            <el-tree
              :data="data"
              node-key="id"
              default-expand-all
              :expand-on-click-node="false">
              <span class="custom-tree-node" slot-scope="{ node, data }">
                <span>{{ node.label }}</span>
                <span>
                  <el-button
                    type="text"
                    size="mini"
                    @click="() => append(data)">
                    Append
                  </el-button>
                  <el-button
                    type="text"
                    size="mini"
                    @click="() => remove(node, data)">
                    Delete
                  </el-button>
                </span>
              </span>
            </el-tree>
          </el-col>
        </el-row>
      </el-tab-pane>
      <el-tab-pane label="年度费用预算类型">
        <div class="budgetType">
          <div class="left">
            <div>
              <el-date-picker
                v-model="year"
                type="year"
                size="small"
                style="width: 360px;margin-bottom: 20px;"
                placeholder="选择年">
              </el-date-picker>
            </div>
            <div>
              <el-select v-model="value" placeholder="请选择" size="small" style="width: 360px;margin-bottom: 20px;">
                <el-option
                  v-for="item in options"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value">
                </el-option>
              </el-select>
            </div>
            <div style="margin-right: 20px;">
              <el-tree
                :data="data1"
                node-key="id"
                default-expand-all
                :expand-on-click-node="false">
                <span class="custom-tree-node" slot-scope="{ node, data }">
                  <span>{{ node.label }}</span>
                  <!-- <span>{{ node }}</span> -->
                  <span>
                    <el-button
                      v-if="node.level=='1'"
                      type="text"
                      size="mini"
                      @click="() => append(data)">
                      关联部门
                    </el-button>
                    <el-button
                      v-else
                      type="text"
                      size="mini"
                      @click="() => remove(node, data)">
                      查看明细
                    </el-button>
                  </span>
                </span>
              </el-tree>
            </div>
          </div>
          <div class="right">
            <h3 style="margin-bottom:20px;">费用归口明细</h3>
            <div>
              <el-link type="primary" :underline="false" @click="openAttributeDialog" style="margin-right: 20px;">新增归口</el-link>
              <el-link type="primary" :underline="false" @click="openAttributeUploadDialog">导入归口明细</el-link>
            </div>
            <el-input v-model="queryAttribute" placeholder="请输入查询归口 " size="small" style="width: 360px;margin-top: 20px;">
              <el-button slot="append" icon="el-icon-search"></el-button>
            </el-input>
            <div>
              <el-tree
                :data="data"
                node-key="id"
                default-expand-all
                :expand-on-click-node="false">
                <span class="custom-tree-node" slot-scope="{ node, data }">
                  <span>{{ node.label }}</span>
                  <span>
                    <el-button
                      type="text"
                      size="mini"
                      @click="() => append(data)">
                      Append
                    </el-button>
                    <el-button
                      type="text"
                      size="mini"
                      @click="() => remove(node, data)">
                      Delete
                    </el-button>
                  </span>
                </span>
              </el-tree>
            </div>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
    <el-dialog
      :title="uploadType=='model'?'导入费用预算类型':'导入归口明细'"
      :visible.sync="dialogVisible"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
      width="640px">
      <el-row :gutter="10">
        <el-col :span="4" style="text-align: right;">导入文件</el-col>
        <el-col :span="20">
          <el-upload
            class="upload-demo"
            action="https://jsonplaceholder.typicode.com/posts/"
            :on-preview="handlePreview"
            :on-remove="handleRemove"
            multiple
            accept=".xlsx"
            :limit="1"
            :file-list="fileList">
            <el-button size="mini" type="primary"  icon="el-icon-upload2">上传文件</el-button>
          </el-upload>
        </el-col>
        <el-col :span="20" :offset="4" style="margin-top:10px;margin-bottom:20px;">
          <el-link type="primary">下载模板</el-link>
        </el-col>
        <el-col :span="20" :offset="2" style="font-size: 15px;line-height: 24px;">
          说明:上传文件必须严格按照模板格式输入数据，重复导入模板将会覆盖上一次数据，系统只保存最近一次的模板数据
        </el-col>
      </el-row>
      <span slot="footer" class="dialog-footer">
        <el-button @click="dialogVisible = false">取 消</el-button>
        <el-button type="primary" @click="dialogVisible = false">确 定</el-button>
      </span>
    </el-dialog>
    <!--新增归口弹窗-->
    <el-dialog
      title="新增归口"
      :visible.sync="attributeDialogVisible"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
      width="480px">
      <div class="attribute">
        <ul>
          <li v-for="(item,index) in attributeList" :key="item.time+index">
            <span>归口名称：</span>
            <el-input v-model="item.value" placeholder="请输入归口名称" style="width: 240px;margin-right: 10px;"></el-input>
            <i class="el-icon-delete" @click="deleteAttribute(index)"></i>
          </li>
        </ul>
        <div>
          <i class="el-icon-circle-plus-outline" @click="addAttribute"></i>
        </div>
      </div>
      <span slot="footer" class="dialog-footer">
        <el-button @click="attributeDialogVisible = false">取 消</el-button>
        <el-button type="primary" @click="submitAttribute">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import 'element-ui/lib/theme-chalk/display.css';
export default {
  name: "budgetTypeSet",
  components: {

  },
  data() {
    return {
      uploadType: 'model', // model 导入费用预算类型  attribute 导入归口明细
      options:[{
        value: '选项1',
        label: '黄金糕'
      }, {
        value: '选项2',
        label: '双皮奶'
      }, {
        value: '选项3',
        label: '蚵仔煎'
      }, {
        value: '选项4',
        label: '龙须面'
      }, {
        value: '选项5',
        label: '北京烤鸭'
      }],
      value: '',
      data: [{
          id: 11,
          label: '一级 1',
          children: [{
            id: 14,
            label: '二级 1-1',
            children: [{
              id: 19,
              label: '三级 1-1-1'
            }, {
              id: 110,
              label: '三级 1-1-2'
            }]
          }]
        }, {
          id: 12,
          label: '一级 2',
          children: [{
            id: 15,
            label: '二级 2-1'
          }, {
            id: 16,
            label: '二级 2-2'
          }]
        }, {
          id: 13,
          label: '一级 3',
          children: [{
            id: 17,
            label: '二级 3-1'
          }, {
            id: 18,
            label: '二级 3-2'
          }]
        }],
      
        data1: [{
          id: 21,
          label: '一级 1',
          children: [{
            id: 24,
            label: '二级 1-1'
          }]
        }, {
          id: 22,
          label: '一级 2',
          children: [{
            id: 25,
            label: '二级 2-1'
          }, {
            id: 26,
            label: '二级 2-2'
          }]
        }, {
          id: 23,
          label: '一级 3',
          children: [{
            id: 27,
            label: '二级 3-1'
          }, {
            id: 28,
            label: '二级 3-2'
          }]
        }],
      dialogVisible: false,
      fileList: [{
          name: 'food.jpeg',
          url: 'https://fuss10.elemecdn.com/3/63/4e7f3a15429bfda99bce42a18cdd1jpeg.jpeg?imageMogr2/thumbnail/360x360/format/webp/quality/100'
        }],
      year:'',
      queryAttribute: "",
      attributeDialogVisible: false,
      attributeList: [
        {
          time: new Date(),
          value: ""
        }
      ]
    }
  },
  watch: {

  },
  computed: {
    
  },
  methods: {
    append(data) {
      // const newChild = { id: id++, label: 'testtest', children: [] };
      // if (!data.children) {
      //   this.$set(data, 'children', []);
      // }
      // data.children.push(newChild);
    },
    openUploadDialogFunc(){
      this.uploadType = 'model';
      this.dialogVisible = true;
    },

    remove(node, data) {
      console.log(node,"node");
      console.log(data,"data");
    },
    handleClose(done) {
      this.$confirm('确认关闭？')
        .then(_ => {
          done();
        })
        .catch(_ => {});
    },
    handleRemove(file, fileList) {
      console.log(file, fileList);
    },
    handlePreview(file) {
      console.log(file);
    },
    handleExceed(files, fileList) {
      // this.$message.warning(`当前限制选择 3 个文件，本次选择了 ${files.length} 个文件，共选择了 ${files.length + fileList.length} 个文件`);
    },
    beforeRemove(file, fileList) {
      return this.$confirm(`确定移除 ${ file.name }？`);
    },
    openAttributeDialog(){
      this.attributeDialogVisible = true
    },
    openAttributeUploadDialog(){
      this.uploadType = 'attribute';
      this.dialogVisible = true;
    },
    deleteAttribute(index){
      if(this.attributeList.length==1){
        this.$message.warning("至少保留一个新增归口")
        return false
      }
      this.attributeList.splice(index,1);
    },
    addAttribute(){
      this.attributeList.push({
        time: new Date(),
        value:''
      })
    },
    submitAttribute(){
      if(this.attributeList.some(item => item.value=='')){
        if(this.attributeList.length==1){
          this.$message.warning("请输入新增归口名称")
        }else{
          this.$message.warning("请输入全部新增归口名称")
        }        
        return false
      }
      this.attributeDialogVisible = false;
    }
  },
  created() {

  },
  mounted() {

  },
  updated() {

  },
  destroyed() {

  }
}
</script>

<style lang="scss" scoped>
.budgetTypeSet{
  .custom-tree-node{
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 14px;
    padding-right: 8px;
  }
  .budgetType{
    min-height: calc(100vh - 192px);
    display: flex;
    .left{
      flex-basis: 50%;
      border-right: 1px solid #DCDFE6;
    }
    .right{
      flex-basis: 50%;
      margin: 20px;
    }
  }
  .attribute{
    li{
      margin-bottom: 12px;
      text-align: center;
      i{
        cursor: pointer;
        color: #f00;
      }
    }
    &>div{
      text-align: center;
      i{
        font-size: 24px;
        color: #1989FA;
        cursor: pointer;
      }
    }
  }
}

@media screen and (max-width: 1200px) {
  .budgetType{
    min-height: calc(100vh - 192px);
    display: flex;
    flex-direction: column;
    .left{
      flex-basis: 100%;
      border-right: none !important;
      min-height: calc(50vh - 96px);
    }
    .right{
      flex-basis: 100%;
      min-height: calc(50vh - 96px);
    }
  }
}
</style>