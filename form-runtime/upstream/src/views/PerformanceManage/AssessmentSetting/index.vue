<template>
  <div class="AssessmentSetting">
    <div class="AssessmentSetting-search">
      <el-form ref="form" :model="searchData" label-width="80px">
        <el-row>
          <el-col :span="6">
            <el-form-item label="考核名称">
              <el-input v-model="searchData.name" clearable></el-input>
            </el-form-item>
          </el-col>
          <el-col :span="2" :offset="1">
            <el-button type="primary" @click="search">查 询</el-button>
          </el-col>
        </el-row>
      </el-form>
    </div>
    <div style="margin-top: 10px;">
      <div style="background: #ffffff;padding: 10px 20px;">
        <el-button type="primary" @click="add">新 增</el-button>
      </div>
      <dy-table maxTableHeight="600" :keys="colKey" :fetchData="fetchData" :list="tableData" :actions="actions"
        style="padding:0px;" ref="usingTable" :isPagination="true" :pagination="searchData">
      </dy-table>
    </div>
    <!--  -->
    <el-dialog :visible="visible" :title="title" width="830px" top="10vh" :close-on-click-modal="false"
      :append-to-body="true" @close='handleClose'>
      <el-form ref="addForm" :model="addForm" :rules="rules" label-width="120px">
        <el-row>
          <el-col :span="24">
            <el-form-item label="考核名称" prop="title">
              <el-input v-model="addForm.title" clearable></el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="时间" prop="targetTime">
              <el-date-picker v-model="addForm.targetTime" value-format="yyyy" type="year" placeholder="选择年" :disabled="title == '修订考核'">
              </el-date-picker>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="考核类型" prop="assessmentCycle">
              <el-select v-model="addForm.assessmentCycle" placeholder="请选择" style="width: 100%;" @change="selectType" :disabled="title == '修订考核'">
                <el-option v-for="item in assessmentCycleList" :key="item.value" :label="item.label" :value="item.value">
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <div v-for="(item, index) in addForm.kpiScoringGroups" :key="index" class="assessmentSetting">
          <div class="assessmentSetting-top">
            <div>
              <!-- <span>{{ item.title }}</span> -->
              <span>{{ titles[item.assessmentCycle] }}</span>
              <span class="top-total" v-if="addForm.kpiScoringGroups.length>1">
                考核总占比：<el-input v-model.number="item.assessmentWeight2" @input="$forceUpdate()" @change="totalGroupWeight" style="width:100px" class="right-aligned-input"> <span slot="suffix">%</span></el-input>
              </span>
              <span class="top-total" v-else>考核总占比：{{ item.assessmentWeight }}%</span>
              <!-- <span style="font-size:14px;color:#1989FA;text-align: end;margin-left:15px">小计：{{ item.assessmentWeight }}%</span> -->
            </div>
          </div>
          <div>
            <div class="assessmentSetting-item" v-for="(k,j) in item.kpiScoringItems" :key="k.kpiScoringType">
              <div class="item-top">
                <div style="width:20%"><span class="tip">{{ k.title }}</span></div>
                <div>
                  <el-form-item label="考核占比：" :prop="`kpiScoringGroups[${index}].kpiScoringItems[${j}].assessmentWeight`" :rules="[{ required: true, message: '请输入', trigger: 'blur' }]">
                    <el-input v-model.number="k.assessmentWeight" @change="totalTypeWeight(item)" class="right-aligned-input"> <span
                        slot="suffix">%</span></el-input>
                  </el-form-item>
                </div>
              </div>
              <div class="item-main">
                <div class="item-main-row" v-for="(q,i) in k.kpiScoringSecondItems" :key="q.kpiScoringType">
                  <div><span v-if="i==0">其中：</span><span v-if="i!=0" style="display: inline-block;width: 42px;"></span><span class="row-tip">{{q.title}}</span></div>
                  <div>
                    <el-form-item label="考核占比：" :prop="`kpiScoringGroups[${index}].kpiScoringItems[${j}].kpiScoringSecondItems[${i}].assessmentWeight`" :rules="[{ required: true, message: '请输入', trigger: 'blur' }]">
                      <el-input v-model.number="q.assessmentWeight" @change="totalWeight(item,k)" class="right-aligned-input"> <span
                          slot="suffix">%</span></el-input>
                    </el-form-item>
                  </div>
                </div>
                <div class="item-main-row total" v-if="k.kpiScoringSecondItems">小计：{{k.totalWeight}}%</div>
              </div>
            </div>
          </div>
        </div>


      </el-form>

      <span slot="footer">
        <el-button @click="handleClose">取 消</el-button>
        <el-button @click="save" type="primary" :loading="submitLoading">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import DyTable from '@/components/DyTable';
export default {
  name: 'AssessmentSetting',
  components: { DyTable },
  data() {
    return {
      titles: { personnel_regularization: '转正考核设置', half_year: '半年考核设置', year: '年终考核设置' },
      loading: false,
      searchData: {
        name: '',
        page: 1,
        pages: 1,
        size: 10,
        total: 0
      },
      tableData: [],
      colKey: {
        title: {
          label: '考核名称',
          showTooltip:true,
          handle: (scope, createElement) => {
            return <span>{scope.row.title}</span>;
            
          }
        },
        assessmentCycle: {
          label: '考核类型',
          handle: (scope, createElement) => {
            if(scope.row.assessmentCycle=='personnel_regularization'){
              return <span>人员转正考核</span>;
            } else if(scope.row.assessmentCycle=='year_and_half_year'){
              return <span>半年及年底考核</span>;
            }else{
              return <span>年度考核</span>;
            }
            
          }
        },
        targetTime:{
          label: '时间',
          handle: (scope, createElement) => {
            return <span>{scope.row.targetTime}年</span>;
          }
        },
        user: {
          label: '更新人',
          width: 120,
          handle: (scope, createElement) => {
            return <span>{scope.row.user?.name}</span>;
          }
        },
        createDate: '更新时间',
        status:{
          label: '状态',
          handle: (scope, createElement) => {
            if(scope.row.active){
              return <span>使用中</span>;
            }else{
              return <span>-</span>;
            }
            
          } 
        }

      },
      actions: [
        {
          label: '修订',
          width: '120px',
          action: (row) => {
            // this.activeRow = row;
            // this.bizId = row.id;
            // this.viewDetail(row);
            this.changeDialog(row);
          }
        },
        {
          label: '删除',
          width: '120px',
          action: (row) => {
            this.detileRow(row);
          }
        }
      ],
      visible: false,
      submitLoading:false,
      title: '新增考核',
      addForm: {
        title: '',
        targetTime: '',
        assessmentCycle: ''
      },
      rules: {
        title: [
            { required: true, message: '请输入考核名称', trigger: 'blur' },
          ],
        targetTime:[{ required: true, message: '请选择考核年份', trigger: 'change' }],
        assessmentCycle:[{ required: true, message: '请选择考核类型', trigger: 'change' }],
      },
      assessmentCycleList: [
        { label: '人员转正考核', value: 'personnel_regularization' },
        { label: '半年及年终考核', value: 'year_and_half_year' },
        { label: '年终考核', value: 'year' }
      ]
    }
  },
  methods: {
    fetchData() {
      const param = {
        data: {
          // customerCode: this.searchData.name,
          title: this.searchData.name,
          enableType: 'enable'
        },
        pagination: true,
        current: this.searchData.pages,
        size: this.searchData.size
      }
      this.$axios.post(
        '/web/plan/api/kpiScoringConfig/list',
        param,
        res => {
          if (res.isSuccess) {
            this.tableData = res.data?.data ? res.data.data : []; // 打开
            this.searchData.total = res.data.total;
          } else {
            this.$message.error(res.message);
          }
        }
      );
    },
    search() {
      this.searchData.pages = 1
      this.fetchData()
    },
    add() {
      this.visible = true
      this.title = '新增考核'
    },
    handleClose() {
     
      if(this.$refs.addForm){
        this.$refs.addForm.resetFields()
      }
      this.addForm={
        title: '',
        targetTime: '',
        assessmentCycle: ''
      }
      this.visible = false
    },
    selectType(val) {
      if (val == 'personnel_regularization') {
        const arr= [
          {
            title: '转正考核设置',
            assessmentWeight: 100,
            assessmentCycle: val,
            kpiScoringItems: [
              {
                title: '试用期考核',
                kpiScoringType: 'daily_evaluation',
                assessmentWeight: 10,
                totalWeight:100,
                kpiScoringSecondItems: [
                  {
                    title: '部门同事评议',
                    kpiScoringType: 'equal_level_evaluation',
                    assessmentWeight: 15,
                  },
                  {
                    title: '部门负责人考评',
                    kpiScoringType: 'supervisor_evaluation',
                    assessmentWeight: 35,
                  },
                  {
                    title: '员工导师考评',
                    kpiScoringType: 'mentor_evaluation',
                    assessmentWeight: 50,
                  },
                ]
              },
              {
                title: '试用期现场述职',
                kpiScoringType: 'probation_period_report_scoring',
                assessmentWeight: 90,
              },
            ]
          }
        ]
        this.$set(this.addForm,'kpiScoringGroups',arr)
      } 
      if (val == 'year_and_half_year') {
        const arr= [
          {
            title: '半年考核设置',
            assessmentWeight: 100,
            assessmentWeight2: 40,
            assessmentCycle: 'half_year',
            kpiScoringItems: [
              {
                title: '年度目标责任书（工作指标）',
                kpiScoringType: 'work_scoring',
                assessmentWeight: 70,
              },
              {
                title: '述职评分',
                kpiScoringType: 'report_scoring_and_manage_scoring',
                assessmentWeight: 20,
                totalWeight: 100,
                kpiScoringSecondItems: [
                  {
                    title: '现场述职',
                    kpiScoringType: 'report_scoring',
                    assessmentWeight: 60,
                  },
                  {
                    title: '年度目标责任书（管理指标）',
                    kpiScoringType: 'manage_scoring',
                    assessmentWeight: 40,
                  }
                ]
              },
              {
                title: '360度评价',
                kpiScoringType: 'three_six_zero_scoring',
                assessmentWeight: 10,
              }
            ]
          },
          {
            title: '年终考核设置',
            assessmentWeight: 100,
            assessmentWeight2: 60,
            assessmentCycle: 'year',
            kpiScoringItems: [
              {
                title: '年度目标责任书（工作指标）',
                kpiScoringType: 'work_scoring',
                assessmentWeight: 50
              },
              {
                title: '述职评分',
                kpiScoringType: 'report_scoring_and_manage_scoring',
                assessmentWeight: 40,
                totalWeight: 100,
                kpiScoringSecondItems: [
                  {
                    title: '现场述职',
                    kpiScoringType: 'report_scoring',
                    assessmentWeight: 60,
                  },
                  {
                    title: '年度目标责任书（管理指标）',
                    kpiScoringType: 'manage_scoring',
                    assessmentWeight: 40,
                  }
                ]
              },
              {
                title: '360度评价',
                kpiScoringType: 'three_six_zero_scoring',
                assessmentWeight: 10,
              }
            ]
          }
        ]
        this.$set(this.addForm,'kpiScoringGroups',arr)
      } else if (val == 'year') {
        const arr = [
          {
            title: '年终考核设置',
            assessmentWeight: 100,
            assessmentCycle: val,
            kpiScoringItems: [
              {
                title: '年度目标责任书（工作指标）',
                kpiScoringType: 'work_scoring',
                assessmentWeight: 50
              },
              {
                title: '述职评分',
                kpiScoringType: 'report_scoring_and_manage_scoring',
                assessmentWeight: 40,
                totalWeight: 100,
                kpiScoringSecondItems: [
                  {
                    title: '现场述职',
                    kpiScoringType: 'report_scoring',
                    assessmentWeight: 60,
                  },
                  {
                    title: '年度目标责任书（管理指标）',
                    kpiScoringType: 'manage_scoring',
                    assessmentWeight: 40,
                  }
                ]
              },
              {
                title: '360度评价',
                kpiScoringType: 'three_six_zero_scoring',
                assessmentWeight: 10,
              }
            ]
          }
        ]
        this.$set(this.addForm,'kpiScoringGroups',arr)
      }
    },
    totalGroupWeight() {
      console.log(this.addForm.kpiScoringGroups,'this.addForm.kpiScoringGroups')
      var total = this.addForm.kpiScoringGroups.reduce((a,b)=>{
        console.log(a, b, 'a,b');
        return a*1+b.assessmentWeight2
      },0)
      console.log(total,'total')
      if(total!==100){
        this.$message.warning('考核总占比之和要为100%!')
      }
    },
    totalWeight(item,k){
      console.log(item,'55555555',k)
      const totalWeight = k.kpiScoringSecondItems.reduce((a,b)=>{
        return a*1+b.assessmentWeight
      },0)
      this.$set(k,'totalWeight',totalWeight)
      if(totalWeight!==100){
        this.$message.warning('考核总占比之和要为100%!')
      }
    },
    totalTypeWeight(item){
      console.log(item,'55555555')
      item.assessmentWeight = item.kpiScoringItems.reduce((a,b)=>{
        return a*1+b.assessmentWeight
      },0)
      if(item.assessmentWeight!==100){ // item.assessmentWeight!==1
        this.$message.warning('考核总占比之和要为100%!')
      }
    },
    save() {
      this.$refs.addForm.validate((valid) => {
        if (valid) {
          var multiple = this.addForm.kpiScoringGroups.length > 1;
          var assessmentWeight = multiple ? 'assessmentWeight2' : 'assessmentWeight';
          var total = 0;
          for (let index = 0; index < this.addForm.kpiScoringGroups.length; index++) {
            const element = this.addForm.kpiScoringGroups[index];
            console.log(element,'element')
            total += element.assessmentWeight2 || 0;
            if (this.addForm.kpiScoringGroups[index].assessmentWeight!==100) {
              this.$message.error(`${element.title || ''}考核总占比之和要为100%!`)
              return
            }
            for (let i=0;i<this.addForm.kpiScoringGroups[index].kpiScoringItems.length;i++) {
              if(this.addForm.kpiScoringGroups[index].kpiScoringItems[i].totalWeight&&this.addForm.kpiScoringGroups[index].kpiScoringItems[i].totalWeight!==100) {
                this.$message.error(`${this.addForm.kpiScoringGroups[0].kpiScoringItems[i].title}的考核总占比之和要为100%!`)
                return
              }
            }
          }
          if (multiple && total !== 100) {
            this.$message.error(`考核总占比之和要为100%!`);
            return;
          }
          this.submitLoading = true
          const param = {
            data:JSON.parse(JSON.stringify(this.addForm))
          }
          param.data.kpiScoringGroups.map(item=>{
            item.assessmentWeight = item[assessmentWeight]/100
            delete item.assessmentWeight2;
            if(item.kpiScoringItems) {
              item.kpiScoringItems.map(k=> {
                k.assessmentWeight = k.assessmentWeight/100
                if(k.kpiScoringSecondItems) {
                  k.kpiScoringSecondItems.map(j=> {
                    j.assessmentWeight = j.assessmentWeight/100
                  })
                }
              })
            }
          })
          console.log(param,'param++++')
          let url = ''
          if(this.title==='新增考核'){
            url = '/web/plan/api/kpiScoringConfig/save'
            param.data.examineStatus = 'pass'
          }else{
            url = '/web/plan/api/kpiScoringConfig/update'
          }
          this.$axios.post(
          url,
            param,
            res => {
              this.submitLoading = false
              if (res.isSuccess) {
                this.$message.success(this.title==='新增考核'?'保存成功！':'修改成功')
                this.handleClose()
                this.search()
              } else {
                this.$message.error(res.message);
              }
            }
          );
        }
      })
    },
    changeDialog(row){
      this.title = '修订考核'
      this.getDetial(row.id)
    },
    getDetial(id){
      this.$axios.post(
            '/web/plan/api/kpiScoringConfig/findById',
            {data:{id}},
            res => {
              if (res.isSuccess) {
               this.addForm = res.data
               this.addForm.targetTime = this.addForm.targetTime+''
               var multi = this.addForm.kpiScoringGroups.length > 1;
               this.addForm.kpiScoringGroups.map(item=>{
                 if (multi) {
                   item.assessmentWeight2 = item.assessmentWeight*100
                   item.assessmentWeight = 100;
                 } else {
                  item.assessmentWeight = item.assessmentWeight*100
                 }
                  if(item.kpiScoringItems){
                    item.kpiScoringItems.map(k=>{
                      k.assessmentWeight = k.assessmentWeight*100
                      if(k.kpiScoringSecondItems){
                        k.kpiScoringSecondItems.map(j=>{
                          j.assessmentWeight = j.assessmentWeight*100
                        })
                        const totalWeight = k.kpiScoringSecondItems.reduce((a,b)=>{
                          return a*1+b.assessmentWeight
                        },0)
                        this.$set(k,'totalWeight',totalWeight)
                      }
                    })
                  }
                })
               this.visible = true
              } else {
                this.$message.error(res.message);
              }
            }
          );
    },
    detileRow(row){
      this.$confirm('确认要删除吗?', '提示', {
        closeOnClickModal: false,
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).then(() => {
        this.$axios.post(
          '/web/plan/api/kpiScoringConfig/delete',
          {data:{id:row.id}},
          res => {
            this.submitLoading = false
            if (res.isSuccess) {
              this.$message.success('删除成功')
              this.search()
              // this.$message.success(this.title==='新增考核'?'保存成功！':'修改成功')
              // this.handleClose()
              // this.search()
            } else {
              this.$message.error(res.message);
            }
          }
        )
      }).catch(() => { });
    }
  }
}
</script>

<style lang="scss" scoped>
.AssessmentSetting {
  &-search {
    background: #ffffff;
    padding-top: 15px;
  }
}

</style>
<style lang="scss">
.assessmentSetting {
  .assessmentSetting-top {
    background-color: #f5f3f3;
    border-radius: 5px;
    padding: 6px;
  }
  .assessmentSetting-item {
    background-color: #f7f7f7;
    border-radius: 5px;
    margin: 10px 20px;
  }
  &-item{
    padding: 5px 30px;
    .item-top{
      display: flex;
      .tip{
        font-size: 16px;
      }
    }
    .item-main{
      padding: 10px 30px;
      &-row{
        display: flex;
        justify-content: space-between;
        .row-tip{
          margin-left: 10px;
          // margin-left: 50px;
        }
        .row-no{
          padding-left: 40px;
        }
      }
      .total{
        margin-left: 470px;
        color: #1989FA;
      }
    }
  }
  &-top {
    padding: 5px;
    font-size: 18px;
    // text-align: center;
    color: #FF3399;
    position: relative;

    .assessmentSetting-right {
      position: absolute;
      top: 0;
      right: 20px;
    }

    .top-total {
      line-height: 28px;
      color: #1989FA;
      margin-left: 400px;
    }
  }

}

</style>