<!--  -->
<template>
  <div class="year-app">
    <div class="searchTop">
      <span v-if="kpiScope == 'my_company'" class="searchItem">
        公司：<el-input
          style="width: 230px; margin-right: 15px"
          placeholder="选择公司"
          clearable
          :disabled="!isTopCompany"
          @focus="selectCompanyFocus"
          @clear="searchData.company.id = ''"
          v-model="searchData.company.name"
        ></el-input>
      </span>
      <span v-if="kpiScope == 'my_company'" class="searchItem">
        部门：<el-input
          clearable
          @focus="selectDepartmentFocus"
          @clear="searchData.department.id = ''"
          v-model="searchData.department.name"
          style="width: 160px; margin-right: 15px"
          placeholder="选择部门"
        ></el-input>
      </span>
      <span v-if="kpiScope == 'my_company'" style="margin-right: 15px" class="searchItem">
        姓名：
        <span class="select-person" @click="selectPersonClick">
          <span v-if="nameTags.length" class="tag-span" @wheel="_=>{_.target.parentElement.scrollLeft -= _.wheelDelta}">
            <el-tag
            :key="tag.id"
            v-for="tag in nameTags"
            closable
            :disable-transitions="true"
            @click.stop
            @close="handleTagClose(tag)">
            {{tag.name}}
          </el-tag>
          </span>
          <span v-else class="placeholder-text">选择姓名</span>
          <i class="el-icon-arrow-down icon"></i>
        </span>
        <!-- <el-input
          clearable
          @focus="selectPersonFocus"
          @clear="searchData.user.id = ''"
          v-model="searchData.user.name"
          style="width: 130px; margin-right: 15px"
          placeholder="选择姓名"
        ></el-input> -->
      </span>
      <span v-if="kpiScope == 'my_company'" class="searchItem">
        岗位类型：<el-select
          v-model="searchData.dutyLevelType"
          placeholder="选择岗位类型"
          style="width: 130px; margin-right:16px;margin-bottom:5px"
          clearable
        >
          <el-option label="一般岗位" value="ordinary" />
          <el-option label="关键岗位" value="important" />
        </el-select>
      </span>
      <span class="searchItem">考核年度：
        <el-date-picker
          v-model="searchData.targetTime"
          type="year"
          ref="datepicker"
          placeholder="选择日期"
          format="yyyy"
          value-format="yyyy"
          clearable
          style="width: 150px; margin-right:16px;margin-bottom:5px"
        ></el-date-picker
      ></span><br/>
      <span v-if="kpiScope == 'my_company'" class="searchItem">
        {{searchType == 'year' ? '目标责任书考核类型' : '考核周期'}}：<el-select
          v-model="searchData.assessmentCycle"
          placeholder="选择考核周期"
          style="width: 120px; margin-right:16px;margin-bottom:5px"
          clearable
        >
          <el-option label="半年" v-if="searchType != 'year'" value="half_year" />
          <el-option label="年终" value="year" />
          <el-option label="半年和年终" v-if="searchType != 'halfYearAndYear'" value="year_and_half_year" />
        </el-select>
      </span>
      <el-button type="primary" @click="getTableData(1)">查询</el-button>
      <el-button type="primary" @click="exportTableData('half_year')" v-if="searchType == 'year' && searchData.assessmentCycle != '' && searchData.assessmentCycle != 'year'">导出半年</el-button>
      <el-button type="primary" @click="exportTableData('year')" v-if="searchType == 'year' && searchData.assessmentCycle != ''">导出全年</el-button>
    </div>
    <div class="content">
      <el-table
        size="small"
        class="dytable-view-container2"
        :max-height="'560px'"
        :header-row-style="tableHeaderRowStyle"
        border
        :data="tableData"
        :span-method="objectSpanMethod"
        :row-class-name="tableRowClassName"
        style="width: 100%; margin-top: 15px"
      >
        <el-table-column prop="companyName" label="公司" width="180" align="center"/>
        <el-table-column prop="depName" label="部门" align="center"/>
        <el-table-column prop="userName" label="姓名" align="center"/>
        <el-table-column prop="targetTime" label="考核年度" align="center"/>
        <el-table-column prop="assessmentCycle2" label="考核周期" align="center"/>
        <el-table-column prop="workScore" width="119" label="任务完成情况得分" align="center">
           <template slot-scope="scope">
             <el-button v-if="isBtnPermission('/manpowerResource/performanceManage/yearlyPerf/workScoreBtn') && scope.row.workScore != '-'"
             type="text" @click="viewDetailHandle(scope.row, 'work_scoring')">{{ scope.row.workScore }}</el-button>
             <span v-else>{{ scope.row.workScore }}</span>
           </template>
        </el-table-column>
        <el-table-column prop="reportScore" label="述职得分" align="center">
          <template slot-scope="scope">
              <el-button v-if="isBtnPermission('/manpowerResource/performanceManage/yearlyPerf/reportScoreBtn') && scope.row.reportScore != '-'"
               type="text" @click="viewDetailHandle(scope.row, 'report_scoring_and_manage_scoring')">{{ scope.row.reportScore }}</el-button>
              <span v-else>{{ scope.row.reportScore }}</span>
           </template>
        </el-table-column>
        <el-table-column prop="threeScore" label="360度考评得分" align="center">
            <template slot-scope="scope">
              <el-button v-if="isBtnPermission('/manpowerResource/performanceManage/yearlyPerf/threeScoreBtn') && scope.row.threeScore != '-'"
              type="text" @click="viewDetailHandle(scope.row, 'three_six_zero_scoring')">{{ scope.row.threeScore }}</el-button>
              <span v-else>{{ scope.row.threeScore }}</span>
           </template>
        </el-table-column>
        <el-table-column prop="totalScore2" label="总得分" align="center"/>
        <el-table-column prop="extraPointsValue" label="集团总经理加分" align="center">
          <template slot-scope="scope">
            <span>{{ scope.row.extraPointsValue }}</span>
            <i
              v-if="(mergeRow != false) && (kpiScope != 'personal')"
              v-permission="'/manpowerResource/performanceManage/yearlyPerf/editButton'"
              class="el-icon-edit-outline editIcon"
              title="修改"
              @click="editIconClick(scope)"
            ></i>
          </template>
        </el-table-column>
        <el-table-column prop="deductPointsValue" label="集团总经理减分" align="center">
          <template slot-scope="scope">
            <span>{{ scope.row.deductPointsValue }}</span>
            <i
              v-if="(mergeRow != false) && (kpiScope != 'personal')"
              v-permission="'/manpowerResource/performanceManage/yearlyPerf/editButton'"
              class="el-icon-edit-outline editIcon"
              title="修改"
              @click="editIconClick(scope)"
            ></i>
          </template>
        </el-table-column>
        <el-table-column prop="rewardPonitsValue" label="考核组加分" align="center">
          <template slot-scope="scope">
            <span>{{ scope.row.rewardPonitsValue }}</span>
            <i
              v-if="(mergeRow != false) && (kpiScope != 'personal')"
              v-permission="'/manpowerResource/performanceManage/yearlyPerf/editButton'"
              class="el-icon-edit-outline editIcon"
              title="修改"
              @click="editIconClick(scope)"
            ></i>
          </template>
        </el-table-column>
        <el-table-column prop="punishPonitsValue" label="考核组减分" align="center">
          <template slot-scope="scope">
            <span>{{ scope.row.punishPonitsValue }}</span>
            <i
              v-if="(mergeRow != false) && (kpiScope != 'personal')"
              v-permission="'/manpowerResource/performanceManage/yearlyPerf/editButton'"
              class="el-icon-edit-outline editIcon"
              title="修改"
              @click="editIconClick(scope)"
            ></i>
          </template>
        </el-table-column>
        <el-table-column prop="totalScore" label="最终得分" align="center">
          <template slot-scope="scope">
            <span style="text-align: center">{{ scope.row.totalScore }}</span>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        style="margin-top: 20px; text-align: right"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
        :current-page="pagination.current"
        :page-sizes="[10, 20, 50]"
        :page-size="pagination.size"
        layout="total, sizes, prev, pager, next, jumper"
        :total="pagination.total"
      >
      </el-pagination>
    </div>
    <el-dialog
      v-if="dialogVisible"
      :title="pointLabel"
      :visible="dialogVisible"
      width="500px"
      @close="dialogVisible = false"
      :close-on-click-modal="false"
    >
      <el-form
        ref="form1"
        :model="formData"
        :rules="formRules"
        label-width="150px"
      >
        <el-form-item label="分数:" prop="name">
          <el-input-number
            class="my_number_input"
            v-model="editRow[editProperty]"
            :controls="false"
            placeholder="输入分数"
            style="width: 200px"
          ></el-input-number>
        </el-form-item>
      </el-form>
      <span slot="footer" class="dialog-footer">
        <el-button @click="_=>{dialogVisible = false}">取 消</el-button>
        <el-button type="primary" @click="confirmClick">确 定</el-button>
      </span>
    </el-dialog>
    <el-dialog v-if="viewDetailVisible" :title="viewDetailTitle" :visible="viewDetailVisible"
      width="750px" @close="viewDetailVisible = false" :close-on-click-modal="false">
      <dy-table :keys="dyTableKeys" :actions="actionKey" :list="tableDataList" :fetchData="_=>_" ref="InfoTable"
        :isPagination="false" :pagination="detailPagination"></dy-table>
      <span slot="footer" class="dialog-footer">
        <el-button @click="_=>{viewDetailVisible = false}">取 消</el-button>
        <el-button @click="_=>{viewDetailVisible = false}" type="primary">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
import {deepClone} from '@/utils';
import DyTable from '@/components/DyTable'
import { localstorageGet } from '@/utils/auth';
export default {
  name: "",
  components: { DyTable },
  props: ['mergeRow', 'kpiScope', 'searchType'],
  data() {
    return {
      viewDetailTitle: '详情',
      viewDetailVisible: false,
      dyTableKeys: {
        title: '标题',
        // totalScore: '分数',
        totalScore: {
          label: '分数',
          handle: (scope, createElement) => {
            return <span>{scope.row.totalScore || '-'}</span>;
          }
        },
        createDate: '提交时间',
        registered: {
          label: '操作',
          width: '80',
          handle: (scope, createElement) => {
            console.log(scope, 'scope');
            return <el-button type="text" onClick={() => {this.previewHandle(scope.row) }}>查看</el-button>;
          }
        }
      },
      actionKey: [
        // {
        //   label: '查看',
        //   width: '80',
        //   // actionFixed:'right',
        //   action: row => {
        //     this.previewHandle(row);
        //   }
        // }
      ],
      tableDataList: [],
      isTopCompany: false,
      nameTags: [],
      originList: [],
      pointLabel: '',
      searchData: {
        company: {
          id: "",
          name: "",
        },
        department: {
          id: "",
          name: "",
        },
        user: {
          id: "",
          name: "",
        },
        users: [
          { id: "", name: "" },
          { id: "", name: "" },
        ],
        dutyLevelType: "",
        targetTime: "",
        assessmentCycle: ""
      },
      dialogVisible: false,
      pagination: {
        current: 1,
        size: 10,
        total: 10,
      },
      detailPagination: {
        total: 0,
        pages: 1,
        size: 10
      },
      formData: {
        number: undefined,
      },
      formRules: {
        number: [{ required: true, trigger: "blur", message: "分数不能为空" }],
      },
      targetTime: "",
      tableData: [],
      editRow: {},
      editProperty: ''
    };
  },
  methods: {
    previewHandle(row) {
      this.$fm.show("flowDetail", {
        data: {
          // id: row.id, // 流程实例id
          flowInstanceBizRelevanceList: [{
            otherBiz: undefined, // 业务类型
            otherBizId: row.id // 业务id
          }]
        }
      });
    },
    viewDetailHandle(row, type) {
      var enums = {work_scoring:'任务完成情况得分',report_scoring_and_manage_scoring:'述职得分',three_six_zero_scoring:'360度考评得分'};
      this.viewDetailTitle = enums[type];
      console.log(row, type, 'row, type');
      this.$axios.post(
        '/web/plan/api/kpiTotalScore/findDetailsById',
        {
          data: {
            id: row.listId,
            kpiTotalScoreGroupId: row.id,
            kpiScoringType: type,
            assessmentCycle: row.assessmentCycleType
          },
          pagination: true,
          current: 1,
          size: 22
        },
        (res) => {
          if (res.isSuccess) {
            var { data } = res;
            this.detailPagination.total = res.total;
            this.tableDataList = data || [];
            this.viewDetailVisible = true;
          }
        }
      );
    },
    isBtnPermission(value) {
      return this.$store.getters.btnPermissionList.some(item => item.href == value);
    },
    tableHeaderRowStyle({ columnIndex }) {
      return { backgroundColor: '#f0f9eb !important', color: '#333 !important' };
    },
    tableRowClassName({ row, rowIndex }) {
      console.log(rowIndex, 'rowIndex');
      if (row.highlightIndex % 2 != 0) {
        return 'row1';
      } else {
        return 'row2';
      }
      return '';
    },
    handleTagClose(tag) {
      console.log(tag, 'tag');
      console.log(this.nameTags.indexOf(tag), 'this.nameTags.indexOf(tag)');
      this.nameTags.splice(this.nameTags.indexOf(tag), 1);
    },
    selectPersonClick() {
      console.log('selectPersonClick');
      this.selectPersonFocus();
    },
    exportTableData(exportCycle) {
      var data = this.getTableData(1);
      if (exportCycle) {
        data.exportCycle = exportCycle;
      }
      const param = {
        data,
        pagination: false
      };
      this.$axios.post(
        '/web/plan/api/kpiTotalScore/export',
        param,
        res => {
          if (res) {
            const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
            // const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' });
            const link = document.createElement('a');
            link.style.display = 'none';
            link.href = URL.createObjectURL(blob);
            link.download = '年度考核汇总.xlsx';
            link.click();
            // document.body.removeChild(link);
            window.URL.revokeObjectURL(link.href);
            link.remove();
          }
        }, '', { responseType: 'blob' }
      );
    },
    getTableData(current) {
      var data = {
        company: {
          id: this.searchData.company.id
        },
        department: {
          id: this.searchData.department.id
        },
        userIds: this.nameTags.map(i => i.id),
        dutyLevelType: this.searchData.dutyLevelType || undefined,
        targetTime: this.searchData.targetTime || undefined,
        assessmentCycle: this.searchData.assessmentCycle || undefined,
        kpiScope: this.kpiScope,
        mergeRow: this.mergeRow
      };
      this.$axios.post(
        '/web/plan/api/kpiTotalScore/list',
        {
          data,
          pagination: true,
          current: current || this.pagination.current,
          size: this.pagination.size
        },
        (res) => {
          if (res.isSuccess) {
            var { data } = res;
            this.pagination.total = res.total;
            this.originList = data || [];
            this.tableData = this.splitData(data || []);
          } else {
            this.tableData = [];
          }
        }
      );
      return data;
    },
    splitData(data) {
      var newArr = [];
      data.forEach((item, index) => {
        var length = item.kpiTotalScoreGroups.length;
        var sortObj = { year: 2, half_year: 1 };
        item.kpiTotalScoreGroups.sort((a, b) => sortObj[a.assessmentCycle] - sortObj[b.assessmentCycle]);
        for (let i = 0;i < length;i++) {
          var j = item.kpiTotalScoreGroups[i];
          var obj = {
            highlightIndex: index,
            listId: item.id,
            id: j.id,
            companyName: item.companyName,
            depName: item.depName,
            userName: item.userName,
            targetTime: item.targetTime,
            assessmentCycle2: ({ year: '年终', half_year: '半年' })[j.assessmentCycle],
            assessmentCycleType: j.assessmentCycle,
            workScore: j.kpiTotalScoreItem?.find(i => i.kpiScoringType == 'work_scoring')?.totalScore || '-',
            reportScore: j.kpiTotalScoreItem?.find(i => i.kpiScoringType == 'report_scoring_and_manage_scoring')?.totalScore || '-',
            threeScore: j.kpiTotalScoreItem?.find(i => i.kpiScoringType == 'three_six_zero_scoring')?.totalScore || '-',
            totalScore2: j.totalScore || '-',
            extraPointsValue: j.extraPointsValue || '-',
            deductPointsValue: j.deductPointsValue || '-',
            rewardPonitsValue: j.rewardPonitsValue || '-',
            punishPonitsValue: j.punishPonitsValue || '-',
            totalScore: item.totalScore || '-'
          };
          if (i == 0) {
            obj.rowSpan = length;
          }
          newArr.push(obj);
        };
      });
      console.log(newArr, 'newArr');
      return newArr;
    },
    objectSpanMethod({ row, column, rowIndex, columnIndex }) {
      if (
        columnIndex === 0 ||
        columnIndex === 1 ||
        columnIndex === 2 ||
        columnIndex === 3 ||
        columnIndex === 13
      ) {
        if (row.rowSpan) {
          return {
            rowspan: row.rowSpan,
            colspan: 1
          };
        } else {
          return {
            rowspan: 0,
            colspan: 0
          };
        }
      }
    },
    selectCompanyFocus() {
      this.$fm.show("orgTree", { type: "onlyCompany" }).then((dialog) => {
        dialog.$on("confirmed", (res) => {
          this.searchData.company.id = res.id;
          this.searchData.company.name = res.name;
        });
      });
    },
    selectDepartmentFocus() {
      this.$fm.show("orgTree", { type: "department", filterId: this.searchData.company.id }).then((dialog) => {
        dialog.$on("confirmed", (res) => {
          this.searchData.department.id = res.id;
          this.searchData.department.name = res.name;
        });
      });
    },
    selectPersonFocus() {
      var filterId = this.searchData.department.id ? this.searchData.department.id : this.searchData.company.id;
      this.$fm.show("orgTree", { type: "multiPerson", filterId, selectValue: this.nameTags }).then((dialog) => {
        dialog.$on("confirmed", (res) => {
          console.log(res, 'ress')
          this.nameTags = res || [];
          // this.searchData.user.id = res.id;
          // this.searchData.user.name = res.name;
        });
      });
    },
    confirmClick() {
      this.$refs.form1.validate((valid) => {
        if (valid) {
          var row = this.editRow;
          var findRow = this.originList.find(i => i.id == row.listId);
          findRow = deepClone(findRow);
          var scoreItem = findRow.kpiTotalScoreGroups.find(j => j.id == row.id);
          scoreItem[this.editProperty] = row[this.editProperty];
          this.$axios.post(
            '/web/plan/api/kpiTotalScore/update',
            {
              data: findRow
            },
            (res) => {
              if (res.isSuccess) {
                this.$message.success('修改成功');
                this.dialogVisible = false;
                this.getTableData();
              }
            }
          );
        } else {
          console.log('error submit!!');
          return false;
        }
      });
    },
    editIconClick(scope) {
      console.log(scope, "scope");
      var { column, row } = scope;
      this.editRow = deepClone(row);
      this.editProperty = column.property;
      this.editRow[column.property] = this.editRow[column.property] == '-' ? undefined : this.editRow[column.property];
      this.pointLabel = column.label;
      // this.formData.number = row[column.property] == '-' ? undefined : row[column.property];
      this.dialogVisible = true;
    },
    handleSizeChange(val) {
      console.log(`每页 ${val} 条`);
      this.pagination.size = val;
      this.getTableData();
    },
    handleCurrentChange(val) {
      console.log(`当前页: ${val}`);
      this.pagination.current = val;
      this.getTableData();
    },
  },
  computed: {},
  watch: {},
  created() {
    // window.abb = this;
    this.isTopCompany = localstorageGet('topCompanyId') == localstorageGet('companyId');
    if (!this.isTopCompany) {
      this.searchData.company.id = localstorageGet('companyId');
      this.searchData.company.name = localstorageGet('companyName');
    }
    this.getTableData();
  },
  mounted() {},
};
</script>
<style lang='scss' scoped>
::v-deep {
  .my_number_input input{
    text-align: center;
  }
}
.dytable-view-container2 {
  border-color: rgb(165, 160, 160) !important;
  border-right: solid 1px;
  border-bottom: solid 1px;
}
::v-deep .dytable-view-container2 tbody td{
  border-color: rgb(165, 160, 160);
}
::v-deep .dytable-view-container2 thead th{
  border-color: rgb(165, 160, 160) !important;
}
::v-deep .dytable-view-container2 .row1 {
  background: #ecf5ff
}
::v-deep .dytable-view-container2 .row2 {
  background: #f9f9f9; //#f3f3f3;
}
::v-deep .dytable-view-container2 .el-table__row:hover > td {
  background-color: transparent !important;
}
.el-tag + .el-tag {
    margin-left: 4px;
}
.searchTop{
  .searchItem{
    display: inline-block;
    margin-top: 5px;
  }
  .select-person{
    display: inline-block;
    height: 29px;
    line-height: 28px;
    width: 220px;
    border: 1px solid #dcdfe6;
    border-radius: 4px;
    padding-left: 5px;
    padding-right: 18px;
    white-space: nowrap;
    // overflow: hidden;
    cursor: text;
    .placeholder-text{
      // margin-left: 10px;
      display: inline-block;
      width: 100%;
      font-size: 12px;
      color: #c0c4cc;
      user-select: none;
    }
    .tag-span{
      display: inline-block;
      width: 100%;
      overflow: hidden;
      vertical-align: middle;
    }
    .icon {
      vertical-align: middle;
      font-size: 14px;
      color: #1989fa;
      cursor: pointer;
    }
  }
}
.content {
  .editIcon {
    color: #1989fa;
    font-size: 14px;
    cursor: pointer;
    margin-left: 4px;
    vertical-align: super;
    position: absolute;
    top: 3px;
    right: 10px;
    &:hover {
      color: blue;
    }
  }
}
</style>
