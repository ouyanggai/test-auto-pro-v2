<!--
 * @Descripttion: your project
 * @Author: zhengzetao
 * @Date: 2024-05-15 15:07:55
-->
<template>
  <el-popover
    placement="bottom"
    :disabled="disabled"
    v-model="visibletest"
    @show="openProjectTree">
    <el-input clearable size="mini" placeholder="输入项目名称搜索" suffix-icon="el-icon-search" v-model.trim="filterText"
    style="margin-bottom:5px;display:flow-root;"></el-input>
    <div style="display:inline-block;width:350px;vertical-align: top;border-right:1px solid #c4d1dd;
    max-height:500px;overflow: scroll;">
      <el-tree
      ref="myTree"
      node-key="id"
      :data="treedata"
      :highlight-current="true"
      :expand-on-click-node="false"
      :props="defaultProps"
      default-expand-all
      @node-click="handleNodeClick">
      </el-tree>
    </div>
    <div style="display:inline-block;width:350px;border-left:1px solid #c4d1dd;transform: translate(-1px,0);
    max-height:500px;overflow: scroll;
    ">
      <ul>
        <template v-for="i in selectTreeNode.projectList">
          <li class="clearfix liHover"  :key="i.id" @click="handleLiClick(i)" v-if="i.showItem"
          style="padding:2.5px;cursor:pointer;line-height:26px" :style="{backgroundColor:selectItem == i.id ? '#edf6ff' : 'white'}">
            <span style="float:left;padding-left:4px" :title="i.name" class="rightProjectName">{{ i.name}}</span>
            <span style="float:right;padding-right:4px">
              {{i.relationType == 'public_project' ? '集团公共' : '公司私有'}}
            </span>
          </li>
        </template>
      </ul>
    </div>
    <slot slot="reference" :viewName='viewData.viewName'></slot>
  </el-popover>
</template>

<script>
import Api from '@/api';
import { localstorageGet,localstorageSet } from '@/utils/auth';
import { parseJsonObject } from '@/utils/parse-value';

export default {
  name: '',
  model: {
    prop: 'myValue', // value
    event: 'changeMyValue' // input
  },
  components: {},
  props: {
    myValue: { // value
      type: [String], // [Array, String, Number]
      default() {
        return '';
      }
    },
    disabled: { // value
      type: [Boolean], // [Array, String, Number]
      default() {
        return false;
      }
    }
  },
  data () {
    return {
      filterText: '',
      projectArr: [],
      oldVlue: this.myValue,
      viewData: {
        viewName: ''
      },
      treedata: [],
      // treedata: [{
      //   id: 1,
      //   name: '润世华新能源控股集团有限公司',
      //   projectList: [{ id: '11', name: 'xx1项目' }, { id: '22', name: 'xx2项目' }], // 项目list
      //   childrenList: [{ // 子公司list
      //     id: 12,
      //     name: '上海斯能投资有限公司',
      //     projectList: [{ id: '33', name: 'xx3项目' }, { id: '44', name: 'xx4项目' }] // 项目list
      //   }]
      // }],
      selectTreeNode: {},
      selectItem: '',
      visibletest: false,
      defaultProps: {
        children: 'childrenList',
        label: 'name'
      }
    };
  },
  computed: {
    compMyValue() {
      return this.myValue;
    }
  },
  watch: {
    filterText(val) {
      this.handleFilter(val);
    },
    compMyValue(newVal, oldVal) {
      console.log('项目-w====atch22222',newVal)
      console.log('this.treedata',this.treedata)
      this.handleInitTree();
      // console.log('oldVal', oldVal);
      // if (!oldVal) return;
      var fidNum = 0;
      let newValId = parseJsonObject(newVal).id || '';
      console.log(22,newValId)
      if (this.treedata.length) {
        const fn = (sr) => {
          sr.forEach(el => {
            console.log('el.projectList',el.projectList)
            var fid = el.projectList.find(i => i.id == newValId);
            console.log('fid',fid)
            if (fid) {
              this.$emit('selectChange', fid);
              this.viewData.viewName = fid.name;
              this.selectTreeNode = el;
              this.selectItem = fid.id;
              fidNum++;
            };
            ((el.childrenList) && (el.childrenList.length > 0)) && fn(el.childrenList);
          });
        };
        fn(this.treedata);
        if (!fidNum) {
          const first = this.treedata[0];
          this.selectTreeNode = first;
          this.$refs.myTree.setCurrentKey(first.id);
          this.viewData.viewName = '';
          this.selectItem = '';
        }
      }
    }
  },
  created() {
    console.log('created22211111444555',this.myValue)
    if (this.myValue) {
      this.handleInitTree();
    }
    // this.handleInitTree();
  },
  mounted() {},
  methods: {
    handleLiClick(val) {
      console.log('handleLiClick-val',val)
      console.log('handleLiClick-val2',this.myValue)
      this.selectItem = val.id;
      const currentValue = parseJsonObject(this.myValue)
      if (currentValue.selectCompany) {
        this.$emit('changeMyValue', JSON.stringify(Object.assign({}, currentValue, {
          id: val.id,
          name: val.name
        })));
      } else {
        this.$emit('changeMyValue', JSON.stringify({
          id: val.id,
          name: val.name
        }));
      }

      // this.$emit('changeMyValue', val.id);
      // console.log('handleLiClick-val',val)
      this.$emit('selectChange', val);
      this.viewData.viewName = val.name;
      this.visibletest = false;
    },
    handleNodeClick(val) {
      this.selectTreeNode = val;
    },
    handleInitTree() {
      console.log('handleInitTree2',this.myValue)
      const currentValue = parseJsonObject(this.myValue)
      let companyId = currentValue.selectCompany ? currentValue.selectCompanyId : localstorageGet('companyId');
      // this.$axios.post('/web/user/api/company/getCompanyTree', { }, // 更换了下面的接口，通过公司id过滤
      this.$axios.post('/web/user/api/company/children', {
        data:{
          id: companyId,
          flag:7
        }
       },
        (res) => {
          if (res.isSuccess) {
            // 处理返回的数据，过滤第一个元素的childrenList
            if(res.data && res.data.length > 0 && res.data[0].childrenList) {
              res.data[0].childrenList = res.data[0].childrenList.filter(item => item.id === companyId);
            }

            this.getAllProject(res.data || []);
          }
        }
      );
    },
    openProjectTree(){
      this.handleInitTree();
    },
    //根据项目id查询项目详情
    getProjectDetail(id) {
      return this.$axios.post(
        Api.myProject.getProjectDetail, {
        data: {
          id: id
        }
      })
    },
    // 是否JSON
    isJSON(str) {
      try {
        const result = JSON.parse(str);
        // JSON 格式必须是一个对象或数组
        return typeof result === 'object' && result !== null;
      } catch (e) {
        return false;
      }
    },
    getAllProject(treeData) {
      console.log('getAllProject')
      var selectTreeId = '';
      function GetAllpromises() {
        const promises = [];
        const fn = (sr) => {
          sr.forEach(el => {
            // promises.push(this.$axios.post('/web/project/api/getProjectVosByCompanyId', { data: { companyId: el.id }},
            promises.push(this.$axios.post('/web/project/api/getProjectVosOfCompanyAndGroup', { data: { companyId: el.id }},
            async (res) => {
                if (res.isSuccess) {
                  // 添加JSON判断是为了兼容生产已有数据（生产只存了id）
                  let getMyNewVal = this.myValue == '' ? this.myValue : this.isJSON(this.myValue) ? JSON.parse(this.myValue) : this.myValue;
                  let JsonMyValue = this.isJSON(this.myValue) ? JSON.parse(this.myValue) : getMyNewVal;

                  // let JsonMyValue = this.myValue == '' ? this.myValue : JSON.parse(this.myValue);
                  let getId = JsonMyValue?.id || JsonMyValue
                  let getDataValue = res.data || [];
                  var fid = getDataValue.find(i => i.id == getId);
                  // var fid = getDataValue.find(i => i.id == JsonMyValue?.id || JsonMyValue);
                  el.projectList = res.data || [];
                  this.projectArr.push(el.projectList);

                  console.log('getAllProject-fid',fid)
                  if (fid) {
                    console.log('fid有值')
                    this.$emit('selectChange', fid);
                    this.viewData.viewName = fid.name;
                    this.selectTreeNode = el;
                    this.selectItem = fid.id;
                    selectTreeId = el.id;
                  } else {
                    console.log('fid为空')
                  }
                }
              }
            ));
            ((el.childrenList) && (el.childrenList.length > 0)) && fn(el.childrenList);
          });
        };
        fn(treeData);
        return promises;
      }
      const getAllpromises = GetAllpromises.bind(this);
      Promise.all(getAllpromises()).then(res => {
        this.treedata = treeData;
        this.handleFilter('');
        this.$nextTick(async () => {
          if (selectTreeId) {
            this.$refs.myTree.setCurrentKey(selectTreeId);
          } else {
            const first = treeData[0];
            this.selectTreeNode = first;
            this.$refs.myTree.setCurrentKey(first.id);
          }
          // 如果发起人选了a公司私有项目，如果下个审核人是b公司，项目列表就没有匹配上已选中的项目，所以增加一层判断，用于回显项目名
          console.log('this.myValue',this.myValue)
          console.log('this.viewData',this.viewData)
          if (this.viewData.viewName == "" && this.myValue) {
            // 添加JSON判断是为了兼容生产已有数据（生产只存了id）
            let JsonMyValue = this.isJSON(this.myValue) ? JSON.parse(this.myValue) : this.myValue;
            // let JsonMyValue = JSON.parse(this.myValue);
            let ProjectDetail = await this.getProjectDetail(JsonMyValue?.id || this.myValue);
            // let ProjectDetail = await this.getProjectDetail(JsonMyValue.id);
            this.viewData.viewName = ProjectDetail.data.name;
          }
        });
      }).catch(err => { console.log(err); });
    },
    handleFilter(val) {
      this.projectArr.forEach(j => {
        j.forEach(i => {
          if (i.name && i.name.includes(val)) {
            i.showItem = true;
          } else {
            i.showItem = false;
          }
        });
      });
    }
  }
};

</script>
<style lang='scss' scoped>
.liHover:hover{
  background-color: #edf6ff !important;
}
.rightProjectName{
  text-overflow: ellipsis;
  white-space: nowrap;
  overflow: hidden;
  width: 78%;
}
</style>
