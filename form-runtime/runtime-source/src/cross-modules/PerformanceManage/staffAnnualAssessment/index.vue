<!--
 * @Author: oygsky
 * @Date: 2025-12-05 16:11:29
 * @LastEditors: oygdeMac-mini.local
 * @LastEditTime: 2025-12-30 10:07:50
 * @Description: 员工年度考核
 * @FilePath: /rsh-cloud-invest-power-system-tempdev-oygdev2/src/views/PerformanceManage/staffAnnualAssessment/index.vue
-->
<template>
  <div class="staff-annual-assessment">
    <div class="search-top">
      <span class="search-item">
        公司：
        <el-input
          style="width: 200px; margin-right: 15px"
          placeholder="选择公司"
          clearable
          @focus="selectCompanyFocus"
          @clear="searchData.companyId = ''"
          v-model="searchData.companyName"
          :disabled="isCompanyDisabled"
        ></el-input>
      </span>
      <span class="search-item">
        部门：
        <el-input
          style="width: 180px; margin-right: 15px"
          placeholder="输入部门"
          clearable
          v-model="searchData.departmentName"
        ></el-input>
      </span>
      <span class="search-item">
        姓名：
        <el-input
          style="width: 150px; margin-right: 15px"
          placeholder="输入姓名"
          clearable
          v-model="searchData.userName"
        ></el-input>
      </span>
      <span class="search-item">
        年度：
        <el-date-picker
          v-model="searchData.year"
          type="year"
          value-format="yyyy"
          placeholder="选择年度"
          style="width: 120px; margin-right: 15px"
          clearable
        >
        </el-date-picker>
      </span>
      <el-button type="primary" @click="handleSearch">查询</el-button>
      <el-button type="primary" @click="handleReset">重置</el-button>
      <el-button type="primary" @click="handleExportAll">导出</el-button>
      <!-- <el-button type="warning" plain @click="handlePreviewNoForm">预览年终考核无表单</el-button> -->
    </div>

    <div class="content">
      <el-table
        size="small"
        class="dytable-view-container"
        :max-height="'600px'"
        border
        :data="tableData"
        style="width: 100%; margin-top: 15px"
      >
        <el-table-column prop="companyName" label="公司" align="center" />
        <el-table-column prop="deptName" label="部门" align="center" />
        <el-table-column prop="userName" label="姓名" align="center">
          <template slot-scope="scope">
            <el-button
              type="text"
              size="small"
              @click="handleViewUserPerformance(scope.row)"
            >
              {{ scope.row.userName }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column prop="year" label="年度" align="center" />
        <el-table-column prop="firstQuarter" label="一季度" align="center">
          <template slot-scope="scope">
            {{
              formatScore(scope.row.firstQuarter) ||
              getQuarterScore(scope.row.kpiGroups, 1) ||
              "-"
            }}
          </template>
        </el-table-column>
        <el-table-column prop="secondQuarter" label="二季度" align="center">
          <template slot-scope="scope">
            {{
              formatScore(scope.row.secondQuarter) ||
              getQuarterScore(scope.row.kpiGroups, 2) ||
              "-"
            }}
          </template>
        </el-table-column>
        <el-table-column prop="thirdQuarter" label="三季度" align="center">
          <template slot-scope="scope">
            {{
              formatScore(scope.row.thirdQuarter) ||
              getQuarterScore(scope.row.kpiGroups, 3) ||
              "-"
            }}
          </template>
        </el-table-column>
        <el-table-column prop="fourQuarter" label="四季度" align="center">
          <template slot-scope="scope">
            {{
              formatScore(scope.row.fourQuarter) ||
              getQuarterScore(scope.row.kpiGroups, 4) ||
              "-"
            }}
          </template>
        </el-table-column>
        <el-table-column prop="reviewScore" label="述职得分" align="center">
          <template slot-scope="scope">
            <el-button
              type="text"
              size="small"
              @click="handleViewReportDetails(scope.row)"
              :style="{ color: hasReviewUnderReview(scope.row) ? 'red' : '' }"
            >
              {{ formatScore(scope.row.reviewScore) || "-" }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column
          prop="finalScore"
          label="年终得分"
          width="100"
          align="center"
        >
          <template slot-scope="scope">
            {{ formatScore(scope.row.finalScore) || "-" }}
          </template>
        </el-table-column>
        <el-table-column prop="signUserName" label="签字确认" align="center">
          <template slot-scope="scope">
            {{
              scope.row.examineStatus === "pass"
                ? scope.row.userName || "-"
                : scope.row.signUserName || "-"
            }}
          </template>
        </el-table-column>
        <el-table-column label="操作" align="center" fixed="right" width="200">
          <template slot-scope="scope">
            <el-button
              v-permission="'/manpowerResource/performanceManage/staffAnnualAssessment/edit'"
              type="text"
              size="small"
              @click="handleEditScore(scope.row)"
              >修改分数</el-button
            >
            <el-button
              type="text"
              size="small"
              @click="handleInitiateConfirm(scope.row)"
              :disabled="isInitiateConfirmDisabled(scope.row)"
            >
              发起年终确认
            </el-button>
            <i
              class="el-icon-info"
              style="color: red; margin-left: 5px; cursor: pointer"
              v-if="hasScoreDiscrepancy(scope.row)"
              @click="handleShowScoreDiff(scope.row)"
            ></i>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        style="margin-top: 20px; text-align: right"
        @size-change="handleSizeChange"
        @current-change="handleCurrentChange"
        :current-page="pagination.current"
        :page-sizes="[10, 20, 50, 100]"
        :page-size="pagination.size"
        layout="total, sizes, prev, pager, next, jumper"
        :total="pagination.total"
      >
      </el-pagination>
    </div>

    <!-- 修改分数弹窗 -->
    <el-dialog
      title="修改分数"
      :visible.sync="editScoreDialogVisible"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="editScoreForm"
        :model="editScoreForm"
        :rules="editScoreRules"
        label-width="120px"
      >
        <el-form-item label="一季度得分" prop="firstQuarter">
          <el-input-number
            v-model="editScoreForm.firstQuarter"
            :controls="false"
            :min="0"
            :max="5"
            :step="0.01"
            :precision="2"
            style="width: 100%"
          ></el-input-number>
        </el-form-item>
        <el-form-item label="二季度得分" prop="secondQuarter">
          <el-input-number
            v-model="editScoreForm.secondQuarter"
            :controls="false"
            :min="0"
            :max="5"
            :step="0.01"
            :precision="2"
            style="width: 100%"
          ></el-input-number>
        </el-form-item>
        <el-form-item label="三季度得分" prop="thirdQuarter">
          <el-input-number
            v-model="editScoreForm.thirdQuarter"
            :controls="false"
            :min="0"
            :max="5"
            :step="0.01"
            :precision="2"
            style="width: 100%"
          ></el-input-number>
        </el-form-item>
        <el-form-item label="四季度得分" prop="fourQuarter">
          <el-input-number
            v-model="editScoreForm.fourQuarter"
            :controls="false"
            :min="0"
            :max="5"
            :step="0.01"
            :precision="2"
            style="width: 100%"
          ></el-input-number>
        </el-form-item>
      </el-form>
      <span slot="footer" class="dialog-footer">
        <el-button @click="editScoreDialogVisible = false">取 消</el-button>
        <el-button type="primary" @click="handleSaveEditScore">确 定</el-button>
      </span>
    </el-dialog>

    <!-- 员工年度绩效考核数据弹窗 -->
    <el-dialog
      :title="currentUserName"
      :visible.sync="userPerformanceDialogVisible"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-table
        size="small"
        border
        :data="userPerformanceList"
        style="width: 100%"
      >
        <el-table-column prop="processName" label="流程名称" align="center">
          <template slot-scope="scope">
            <el-button
              type="text"
              size="small"
              @click="handleViewProcessDetail(scope.row)"
            >
              {{ scope.row.processName }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column prop="score" label="分数" width="150" align="center" />
      </el-table>
    </el-dialog>

    <!-- 述职评分数据弹窗 -->
    <el-dialog
      :title="currentWorkReportUserName"
      :visible.sync="workReportDialogVisible"
      width="700px"
      :close-on-click-modal="false"
    >
      <el-table size="small" border :data="workReportList" style="width: 100%">
        <el-table-column prop="processName" label="流程名称" align="center">
          <template slot-scope="scope">
            <el-button
              type="text"
              size="small"
              @click="handleViewWorkReportDetail(scope.row)"
            >
              {{ scope.row.processName }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column
          prop="assessor"
          label="考核人"
          width="120"
          align="center"
        />
        <el-table-column prop="score" label="分数" width="120" align="center">
          <template slot-scope="scope">
            <span
              :style="{
                color: scope.row.status === 'under_review' ? '#F56C6C' : '#333',
              }"
            >
              {{  formatScore(scope.row.score) || "-" }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100" align="center">
          <template slot-scope="scope">
            <el-tag :type="getWorkReportStatusType(scope.row.status)">
              {{ getWorkReportStatusText(scope.row.status) }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
    <!-- 发起流程弹窗 -->
    <FlowDialog
      ref="flowDialog"
      :visible.sync="flowDialogVisible"
      v-if="flowDialogVisible"
      :flowJson.sync="flowJson"
      :flowType.sync="flowType"
      @success="handleFlowSuccess"
      :closeAll="true"
      :needReSubmit="needReSubmit"
    />

    <!-- 分数对比提示弹窗 -->
    <el-dialog
      title="提示"
      :visible.sync="scoreDiffVisible"
      width="600px"
      append-to-body
    >
      <div
        v-for="(item, index) in scoreDiffList"
        :key="index"
        style="margin-bottom: 20px; font-size: 14px"
      >
        <span style="display: inline-block; width: 250px; text-align: left"
          >{{ item.name }}：<span
            :style="{
              color: item.kpiScore !== item.currentScore ? 'red' : '',
              fontWeight:
                item.kpiScore !== item.currentScore ? 'bold' : 'normal',
            }"
            >{{ item.kpiScore }}</span
          ></span
        >
        <span
          style="
            display: inline-block;
            width: 200px;
            text-align: left;
            margin-left: 20px;
          "
          >当前分数：{{ item.currentScore }}</span
        >
      </div>
      <span slot="footer" class="dialog-footer">
        <el-button type="primary" @click="scoreDiffVisible = false"
          >关闭</el-button
        >
      </span>
    </el-dialog>
  </div>
</template>

<script>
import { localstorageGet } from "@/utils/auth";
import Api from "@/api";
import FlowDialog from "@/views/GroupApproveManage/Submitted/components/FlowDialog.vue";
import XLSX from "@/lib/xlsx/xlsx.full.min.js";

export default {
  name: "StaffAnnualAssessment",
  components: {
    FlowDialog,
  },
  data() {
    return {
      // 查询条件
      searchData: {
        companyId: "",
        companyName: "",
        departmentName: "",
        userName: "",
        year: "",
      },
      // 表格数据
      tableData: [],
      // 分页
      pagination: {
        current: 1,
        size: 20,
        total: 0,
      },
      // 修改分数弹窗
      editScoreDialogVisible: false,
      editScoreForm: {
        userId: "",
        firstQuarter: null,
        secondQuarter: null,
        thirdQuarter: null,
        fourQuarter: null,
      },
      editScoreRules: {
        firstQuarter: [
          {
            type: "number",
            min: 0,
            max: 5,
            message: "请输入0-5之间的分数",
            trigger: "blur",
          },
        ],
        secondQuarter: [
          {
            type: "number",
            min: 0,
            max: 5,
            message: "请输入0-5之间的分数",
            trigger: "blur",
          },
        ],
        thirdQuarter: [
          {
            type: "number",
            min: 0,
            max: 5,
            message: "请输入0-5之间的分数",
            trigger: "blur",
          },
        ],
        fourQuarter: [
          {
            type: "number",
            min: 0,
            max: 5,
            message: "请输入0-5之间的分数",
            trigger: "blur",
          },
        ],
      },
      // 当前用户是否是顶级公司
      isTopCompany: false,
      // 员工年度绩效考核弹窗
      userPerformanceDialogVisible: false,
      currentUserName: "",
      userPerformanceList: [],
      // 述职评分弹窗
      workReportDialogVisible: false,
      currentWorkReportUserName: "",
      workReportList: [],
      // 流程弹窗相关数据
      flowDialogVisible: false,
      flowJson: null,
      businessId: "", // 保存的业务ID
      flowType: "", // 流程类型
      tempFormData: null, // 临时表单数据
      // 分数对比弹窗
      scoreDiffVisible: false,
      scoreDiffList: [],
      // 公司选择是否禁用
      isCompanyDisabled: false,
      // 是否需要重新提交
      needReSubmit: false,
    };
  },
  created() {
    this.searchData.year = this.getDefaultYear();
    this.checkUserCompany();
    this.initCompanyData();
    this.loadData();
  },
  methods: {
    // 初始化公司数据
    initCompanyData() {
      const data = {
        sid: localstorageGet("token"),
        data: {
          type: "company",
        },
      };
      this.$axios.post(
        "/web/user/api/company/getCompanyListOfOnDuty",
        data,
        (res) => {
          if (res.isSuccess && res.data && res.data.length > 0) {
            const userCompanyId = localstorageGet("companyId");
            // 根据 user 状态中的 companyId 匹配
            const currentCompany = res.data.find(
              (item) => item.id === userCompanyId
            );

            if (currentCompany) {
              // 如果 parentId 不为空，说明是子公司，锁定并默认选中
              if (currentCompany.parentId) {
                this.isCompanyDisabled = true;
                this.searchData.companyId = currentCompany.id;
                this.searchData.companyName = currentCompany.name;
              } else {
                // parentId 为空，顶级公司，不锁定
                this.isCompanyDisabled = false;
              }
            }
          }
        }
      );
    },
    // 获取默认年度
    getDefaultYear() {
      const now = new Date();
      // 如果是一季度（1-3月，getMonth()返回0-2），显示上一年
      if (now.getMonth() < 3) {
        return (now.getFullYear() - 1).toString();
      }
      // 否则显示当前年
      return now.getFullYear().toString();
    },
    // 检查用户公司级别
    checkUserCompany() {
      this.isTopCompany =
        localstorageGet("topCompanyId") === localstorageGet("companyId");
      // 不自动填充公司信息，保持查询条件默认不选择
    },
    // 选择公司
    selectCompanyFocus() {
      this.$fm.show("orgTree", { type: "onlyCompany" }).then((dialog) => {
        dialog.$on("confirmed", (res) => {
          this.searchData.companyId = res.id;
          this.searchData.companyName = res.name;
        });
      });
    },
    // 查询
    handleSearch() {
      this.pagination.current = 1;
      this.loadData();
    },
    // 重置
    handleReset() {
      const companyId = this.isCompanyDisabled ? this.searchData.companyId : "";
      const companyName = this.isCompanyDisabled
        ? this.searchData.companyName
        : "";
      this.searchData = {
        companyId,
        companyName,
        departmentName: "",
        userName: "",
        year: this.getDefaultYear(),
      };
      this.handleSearch();
    },
    // 导出全部
    handleExportAll() {
      const year = this.searchData.year || this.getDefaultYear();
      const data = {
        year,
        customerCode:
          localstorageGet("customerCode") || "54e991e06b9b4f55b1747ee71fd0d79a",
      };

      if (this.searchData.companyId) {
        data.companyId = this.searchData.companyId;
      }
      if (this.searchData.departmentName) {
        data.deptName = this.searchData.departmentName;
      }
      if (this.searchData.userName) {
        data.userName = this.searchData.userName;
      }

      // 设置 size 为 10000
      const params = {
        data,
        pagination: true,
        pages: 1,
        size: 10000,
      };

      this.$axios.post(
        "/web/plan/api/annualPerformance/list",
        params,
        (response) => {
          if (response.isSuccess && response.data) {
            this.exportDataToExcel(
              response.data.dataList || [],
              "员工年度考核_全部"
            );
          } else {
            this.$message.error(response.message || "导出失败");
          }
        }
      );
    },
    // 导出数据处理
    exportDataToExcel(dataList, fileName) {
      if (!dataList || !dataList.length) {
        this.$message.warning("暂无数据可导出");
        return;
      }

      // 表头
      const headers = [
        "公司",
        "部门",
        "姓名",
        "年度",
        "一季度",
        "二季度",
        "三季度",
        "四季度",
        "述职得分",
        "年终得分",
        "签字确认",
      ];

      // 数据转换
      const data = dataList.map((row) => {
        return [
          row.companyName || "",
          row.deptName || "",
          row.userName || "",
          row.year || "",
          this.formatScore(row.firstQuarter) ||
            this.getQuarterScore(row.kpiGroups, 1) ||
            "-",
          this.formatScore(row.secondQuarter) ||
            this.getQuarterScore(row.kpiGroups, 2) ||
            "-",
          this.formatScore(row.thirdQuarter) ||
            this.getQuarterScore(row.kpiGroups, 3) ||
            "-",
          this.formatScore(row.fourQuarter) ||
            this.getQuarterScore(row.kpiGroups, 4) ||
            "-",
          this.formatScore(row.reviewScore) || "-",
          this.formatScore(row.finalScore) || "-",
          row.examineStatus === "pass"
            ? row.userName || "-"
            : row.signUserName || "-",
        ];
      });

      // 添加表头
      data.unshift(headers);

      // 创建工作簿
      const wb = XLSX.utils.book_new();
      const ws = XLSX.utils.aoa_to_sheet(data);

      // 设置列宽
      const wscols = [
        { wch: 20 }, // 公司
        { wch: 15 }, // 部门
        { wch: 10 }, // 姓名
        { wch: 10 }, // 年度
        { wch: 10 }, // 一季度
        { wch: 10 }, // 二季度
        { wch: 10 }, // 三季度
        { wch: 10 }, // 四季度
        { wch: 10 }, // 述职得分
        { wch: 10 }, // 年终得分
        { wch: 10 }, // 签字确认
      ];
      ws["!cols"] = wscols;

      // 设置表头背景色
      // 注意：标准 xlsx.js 不支持样式，但尝试添加以防项目支持
      const range = XLSX.utils.decode_range(ws["!ref"]);
      for (let C = range.s.c; C <= range.e.c; ++C) {
        const address = XLSX.utils.encode_cell({ r: 0, c: C });
        if (!ws[address]) continue;

        // 尝试多种样式设置方式，以兼容不同的库版本
        ws[address].s = {
          fill: {
            fgColor: { rgb: "E3F2D9" }, // rgb:227 242 217 -> E3F2D9
          },
          font: {
            bold: true,
          },
          alignment: {
            horizontal: "center",
            vertical: "center",
          },
        };
      }

      XLSX.utils.book_append_sheet(wb, ws, "Sheet1");
      XLSX.writeFile(wb, `${fileName}.xlsx`);
    },
    // 分页大小改变
    handleSizeChange(val) {
      this.pagination.size = val;
      this.loadData();
    },
    // 当前页改变
    handleCurrentChange(val) {
      this.pagination.current = val;
      this.loadData();
    },
    // 加载数据
    loadData() {
      const year = this.searchData.year || this.getDefaultYear();

      const data = {
        year,
        customerCode:
          localstorageGet("customerCode") || "54e991e06b9b4f55b1747ee71fd0d79a",
      };

      if (this.searchData.companyId) {
        data.companyId = this.searchData.companyId;
      }
      if (this.searchData.departmentName) {
        data.deptName = this.searchData.departmentName;
      }
      if (this.searchData.userName) {
        data.userName = this.searchData.userName;
      }

      const params = {
        data,
        pagination: true,
        pages: this.pagination.current,
        size: this.pagination.size,
      };

      // 如果有用户ID过滤条件，添加userIds数组
      // 注意：根据API文档，当前接口是根据userIds查询的，可能需要先获取用户列表
      // 这里暂时使用现有逻辑，实际可能需要调整

      this.$axios.post(
        "/web/plan/api/annualPerformance/list",
        params,
        (response) => {
          if (response.isSuccess && response.data) {
            this.tableData = response.data.dataList || [];
            this.pagination.total = response.data.total || 0;
          } else {
            this.tableData = [];
            this.pagination.total = 0;
            this.$message.error(response.message || "查询失败");
          }
        }
      );
    },
    // 修改分数（季度考核分数为1-5的两位小数）
    handleEditScore(row) {
      this.editScoreForm = {
        userId: row.userId,
        firstQuarter: this.isValidNumber(row.firstQuarter)
          ? Number(row.firstQuarter)
          : undefined,
        secondQuarter: this.isValidNumber(row.secondQuarter)
          ? Number(row.secondQuarter)
          : undefined,
        thirdQuarter: this.isValidNumber(row.thirdQuarter)
          ? Number(row.thirdQuarter)
          : undefined,
        fourQuarter: this.isValidNumber(row.fourQuarter)
          ? Number(row.fourQuarter)
          : undefined,
      };
      this.editScoreDialogVisible = true;
    },
    // 保存修改分数
    handleSaveEditScore() {
      this.$refs.editScoreForm.validate((valid) => {
        if (valid) {
          // 调用更新接口
          const data = {
            userId: this.editScoreForm.userId,
            year: this.searchData.year || new Date().getFullYear(),
          };

          if (
            this.editScoreForm.firstQuarter !== undefined &&
            this.editScoreForm.firstQuarter !== null
          ) {
            data.firstQuarter = this.editScoreForm.firstQuarter;
          }
          if (
            this.editScoreForm.secondQuarter !== undefined &&
            this.editScoreForm.secondQuarter !== null
          ) {
            data.secondQuarter = this.editScoreForm.secondQuarter;
          }
          if (
            this.editScoreForm.thirdQuarter !== undefined &&
            this.editScoreForm.thirdQuarter !== null
          ) {
            data.thirdQuarter = this.editScoreForm.thirdQuarter;
          }
          if (
            this.editScoreForm.fourQuarter !== undefined &&
            this.editScoreForm.fourQuarter !== null
          ) {
            data.fourQuarter = this.editScoreForm.fourQuarter;
          }

          const params = {
            data,
          };

          this.$axios.post(
            "/web/plan/api/annualPerformance/update",
            params,
            (response) => {
              if (response.isSuccess) {
                // 更新本地数据
                const index = this.tableData.findIndex(
                  (item) => item.userId === this.editScoreForm.userId
                );
                if (index !== -1) {
                  this.tableData[index].firstQuarter =
                    this.editScoreForm.firstQuarter;
                  this.tableData[index].secondQuarter =
                    this.editScoreForm.secondQuarter;
                  this.tableData[index].thirdQuarter =
                    this.editScoreForm.thirdQuarter;
                  this.tableData[index].fourQuarter =
                    this.editScoreForm.fourQuarter;
                }
                this.$message.success("修改成功");
                this.editScoreDialogVisible = false;
                this.loadData();
              } else {
                this.$message.error(response.message || "修改失败");
              }
            }
          );
        }
      });
    },
    // 获取流程实例id
    getInstanceId(id, type) {
      const flowInstanceBizRelevanceList = [
        {
          otherBiz: type,
        },
      ];
      const data = {
        useScope: "invest",
        initiator: "all",
        flowInstanceBizRelevanceList,
      };
      return new Promise((resolve, reject) => {
        this.$axios
          .post(Api.schedule.getFlowInstanceList, {
            data,
            size: 1,
            pagination: true,
            pages: 1,
          })
          .then((res) => {
            if (res.isSuccess) {
              let data = res?.data || [];
              if (data.length) {
                resolve(data[0]);
              } else {
                resolve(null);
              }
            } else {
              resolve(null);
            }
          })
          .catch(() => resolve(null));
      });
    },
    // 发起年终确认
    async handleInitiateConfirm(row) {
      if (row.annualAssessmentId) {
        const canProceed = await new Promise((resolve) => {
          const data = {
            initiator: 'all',
            useScope: 'invest',
            flowInstanceBizRelevanceList: [
              {
                otherBiz: 'staff_annual_assessment',
                otherBizId: row.annualAssessmentId,
              },
            ],
          };
          this.$axios.post(
            Api.schedule.getFlowInstanceList,
            {
              data,
              pagination: false,
            },
            (res) => {
              console.log('getFlowInstanceList result:', res);
              if (res.isSuccess && res.data && res.data.length > 0) {
                const flowInstance = res.data[0];
                const currentUserId = this.$store.state.user.userId;
                if (flowInstance.createrId !== currentUserId) {
                  this.$alert(
                    `${flowInstance.initiator} 已经发起了 ${row.userName} 的年终确认单 你不能再发起`,
                    '提示',
                    {
                      confirmButtonText: '确定',
                      type: 'warning',
                    }
                  );
                  resolve(false);
                } else {
                  resolve(true);
                }
              } else {
                resolve(true);
              }
            }
          );
        });

        if (!canProceed) return;
      }

      this.$confirm(`确定要发起 ${row.userName} 的年终确认吗？`, "提示", {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
      })
        .then(async () => {
          // 1. 获取流程模板
          const flowTemplateParam = {
            data: {
              useScope: "invest",
              auditWayList: ["staff_annual_assessment"],
            },
            showMe: true,
            ignoreFormTemplateBizRelevanceData: true,
            ignoreTemplateData: true,
            platformCode: "999999",
            pagination: true,
            pages: 1,
            size: 1,
          };

          const flowTemplateResult = await new Promise((resolve) => {
            this.$axios.post(
              Api.schedule.getFlowTemplateList,
              flowTemplateParam,
              (res) => {
                if (res && res.isSuccess && res.data && res.data.length) {
                  resolve(res.data[0]);
                } else {
                  this.$message.error("暂无员工年终确认流程，请联系管理员");
                  resolve(null);
                }
              }
            );
          });

          if (!flowTemplateResult) return;

          // 2. 准备数据，传递给表单组件
          const year = Number(
            String(
              row.year || this.searchData.year || new Date().getFullYear()
            ).replace("年", "")
          );
          const getQuarterValue = (quarterIndex) => {
            let val = null;
            if (quarterIndex === 1) val = row.firstQuarter;
            if (quarterIndex === 2) val = row.secondQuarter;
            if (quarterIndex === 3) val = row.thirdQuarter;
            if (quarterIndex === 4) val = row.fourQuarter;
            if (val === null || val === undefined || val === "") {
              val = this.getQuarterScore(row.kpiGroups, quarterIndex);
            }
            if (val === null || val === undefined || val === "") {
              return null;
            }
            const num = Number(val);
            return isNaN(num) ? null : num;
          };
          console.log(
            "%c [ getQuarterValue ]-752",
            "font-size:13px; background:pink; color:#bf2c9f;",
            getQuarterValue
          );

          // 检查是否已有流程，如果有则重新发起
          const annualAssessmentId = row.annualAssessmentId || null;
          // 只有特定状态才尝试重新发起
          const realExamineStatus = row.realExamineStatus || "";

          //如果有之前的业务id需要重新发起流程
          this.needReSubmit =
            annualAssessmentId &&
            ["not_submitted", "rejected", "withdraw"].includes(realExamineStatus);

          const dataForForm = {
            userId: row.userId,
            year,
            firstQuarter: getQuarterValue(1),
            secondQuarter: getQuarterValue(2),
            thirdQuarter: getQuarterValue(3),
            fourQuarter: getQuarterValue(4),
            customerCode:
              localstorageGet("customerCode") ||
              "54e991e06b9b4f55b1747ee71fd0d79a",
            companyName: row.companyName || "",
            deptName: row.deptName || row.departmentName || "",
            userName: row.userName || "",
            reviewScore: this.formatScore(row.reviewScore),
            reportScore: this.formatScore(row.reportScore),
            finalScore: this.formatScore(row.finalScore),
            realExamineStatus: row.realExamineStatus,
            annualAssessmentId: row.annualAssessmentId,
            opinion: "",
          };

          // 3. 保存当前行数据，准备发起流程
          this.currentRowData = {
            ...row,
            year: year,
          };

          // 4. 设置 FlowDialog 需要的数据
          this.flowJson = flowTemplateResult;
          this.flowType = "staff_annual_assessment";

          // 将数据存储在组件的临时数据中，供表单组件使用
          this.tempFormData = dataForForm;

          // 打开FlowDialog
          this.flowDialogVisible = true;

          // 等待FlowDialog打开后，将数据存储到FlowDialog实例中
          this.$nextTick(() => {
            if (this.$refs.flowDialog) {
              this.$refs.flowDialog.tempFormData = dataForForm;
            }
          });
        })
        .catch((e) => {
          console.log(
            "%c [ e ]-802",
            "font-size:13px; background:pink; color:#bf2c9f;",
            e
          );
        });
    },
    // 打开年中考核确认单
    handleOpenMidYearConfirm(row) {
      const propData = {
        title: "年中考核确认单",
        companyName: row.companyName,
        departmentName: row.departmentName,
        userName: row.userName,
        year: row.year,
        q1Score: Number(row.q1Score.toFixed(2)),
        q2Score: Number(row.q2Score.toFixed(2)),
      };
      this.$fm.show("midYearConfirm", propData).then((dialog) => {
        dialog.$on("confirmed", (res) => {
          this.$message.success("年中考核已确认");
        });
      });
    },
    // 查看员工年度绩效考核数据
    handleViewUserPerformance(row) {
      this.currentUserName =
        row.userName + " - " + row.year + "年度绩效考核数据";
      // 直接使用接口返回的 kpiGroups 数据
      this.userPerformanceList = [];
      if (row.kpiGroups && Array.isArray(row.kpiGroups)) {
        this.userPerformanceList = row.kpiGroups.map((item) => ({
          id: item.id,
          processName: item.title,
          score: this.isValidNumber(item.score) ? item.score : "-",
        }));
      }
      this.userPerformanceDialogVisible = true;
    },
    // 查看流程详情
    handleViewProcessDetail(row) {
      // 使用 kpiGroups 中的 id 来查看流程详情
      this.$fm.show("flowDetail", {
        data: {
          flowInstanceBizRelevanceList: [{ otherBizId: row.id }],
        },
      });
    },
    // 流程发起成功回调
    handleFlowSuccess() {
      this.loadData();
      this.flowDialogVisible = false;
      this.flowJson = null;
    },
    // 查看述职评分详情
    handleViewWorkReportDetail(row) {
      // 使用 reviewDetails 中的 id 来查看流程详情
      this.$fm.show("flowDetail", {
        data: {
          flowInstanceBizRelevanceList: [{ otherBizId: row.id }],
        },
      });
    },
    // 查看述职报告详情（reviewDetails）
    handleViewReportDetails(row) {
      this.currentWorkReportUserName =
        row.userName + " - " + row.year + "年度述职评分";
      // 直接使用接口返回的 reviewDetails 数据
      this.workReportList = [];
      if (row.reviewDetails && Array.isArray(row.reviewDetails)) {
        this.workReportList = row.reviewDetails.map((item) => ({
          id: item.id,
          processName: item.title || "述职评分",
          assessor: item.assessor || "",
          score: this.isValidNumber(item.score) ? item.score.toString() : "-",
          status: item.status || "not_submitted",
        }));
      }
      this.workReportDialogVisible = true;
    },
    // 预览年终考核无表单
    // handlePreviewNoForm() {
    //   // 通过 $fm.show 打开 flowDetail，展示 StaffAnnualAssessmentNoForm 组件
    //   this.$fm.show('flowDetail', {
    //     data: {
    //       flowInstanceBizRelevanceList: [
    //         {
    //           otherBizId: '6f82585bdc304842b4781c1b79f238d3'
    //         }
    //       ]
    //     }
    //   });
    // },
    // 从kpiGroups中获取指定季度的分数
    getQuarterScore(kpiGroups, quarter) {
      if (!kpiGroups || !Array.isArray(kpiGroups)) {
        return null;
      }

      // 根据targetTime的最后一位数字判断季度
      const quarterMap = {
        1: "1", // 第一季度
        2: "2", // 第二季度
        3: "3", // 第三季度
        4: "4", // 第四季度
      };

      const targetQuarter = quarterMap[quarter];
      if (!targetQuarter) return null;

      // 查找对应季度的数据
      const quarterData = kpiGroups.find((item) => {
        const targetTimeStr = item.targetTime ? item.targetTime.toString() : "";
        return targetTimeStr.endsWith(targetQuarter);
      });

      if (!quarterData) return null;
      if (
        quarterData.totalScore === null ||
        quarterData.totalScore === undefined ||
        quarterData.totalScore === ""
      ) {
        return null;
      }
      return parseFloat(quarterData.totalScore || 0).toFixed(2);
    },
    // 格式化分数，保留两位小数
    formatScore(score) {
      if (score === null || score === undefined || score === "") {
        return null;
      }
      const num = parseFloat(score);
      if (isNaN(num)) return null;
      // 使用 Math.round 进行四舍五入
      return (Math.round((num + Number.EPSILON) * 100) / 100).toFixed(2);
    },
    // 获取述职报告状态类型
    getWorkReportStatusType(status) {
      const statusMap = {
        not_submitted: "info", // 未提交审核
        under_review: "warning", // 审核中
        rejected: "danger", // 驳回
        pass: "success", // 已通过审核
        finish: "success", // 已完成
      };
      return statusMap[status] || "info";
    },
    // 获取述职报告状态文本
    getWorkReportStatusText(status) {
      const statusMap = {
        not_submitted: "未提交",
        under_review: "审核中",
        rejected: "已驳回",
        pass: "已通过",
        finish: "已完成",
      };
      return statusMap[status] || "未知状态";
    },
    // 获取审核状态文本
    getExamineStatusText(status) {
      const statusMap = {
        not_submitted: "未提交",
        under_review: "审核中",
        rejected: "已驳回",
        pass: "已通过",
        finish: "已完成",
      };
      return statusMap[status] || status || "";
    },
    // 检查是否有分数差异
    hasScoreDiscrepancy(row) {
      if (!row.kpiGroups || !Array.isArray(row.kpiGroups)) return false;

      const quarterMap = {
        1: "firstQuarter",
        2: "secondQuarter",
        3: "thirdQuarter",
        4: "fourQuarter",
      };

      for (const item of row.kpiGroups) {
        if (!item.targetTime) continue;
        const targetTimeStr = String(item.targetTime);
        const quarter = targetTimeStr.slice(-1);
        const field = quarterMap[quarter];

        if (field) {
          const currentScore = row[field];
          const originalScore = item.score;

          // 只有当两个分数都是有效数字时才进行比对
          if (
            this.isValidNumber(currentScore) &&
            this.isValidNumber(originalScore)
          ) {
            const s1 = parseFloat(currentScore).toFixed(2);
            const s2 = parseFloat(originalScore).toFixed(2);
            if (s1 !== s2) {
              return true;
            }
          }
        }
      }
      return false;
    },
    // 检查 reviewDetails 中是否有正在审核的数据
    hasReviewUnderReview(row) {
      if (!row.reviewDetails || !Array.isArray(row.reviewDetails)) return false;
      return row.reviewDetails.some((item) => item.status === "under_review");
    },
    // 检查是否为有效数字
    isValidNumber(val) {
      if (val === null || val === undefined || val === "") return false;
      return !isNaN(parseFloat(val));
    },
    // 显示分数对比弹窗
    handleShowScoreDiff(row) {
      this.scoreDiffList = [];
      if (!row.kpiGroups || !Array.isArray(row.kpiGroups)) return;

      const quarterNameMap = {
        1: "一季度绩效考核分数",
        2: "二季度绩效考核分数",
        3: "三季度绩效考核分数",
        4: "四季度绩效考核分数",
      };

      const quarterFieldMap = {
        1: "firstQuarter",
        2: "secondQuarter",
        3: "thirdQuarter",
        4: "fourQuarter",
      };

      ["1", "2", "3", "4"].forEach((q) => {
        const item = row.kpiGroups.find(
          (g) => g.targetTime && String(g.targetTime).endsWith(q)
        );
        if (item && this.isValidNumber(item.score)) {
          const currentScore = row[quarterFieldMap[q]];
          this.scoreDiffList.push({
            name: quarterNameMap[q],
            kpiScore: parseFloat(item.score).toFixed(2),
            currentScore: this.isValidNumber(currentScore)
              ? parseFloat(currentScore).toFixed(2)
              : "-",
          });
        }
      });

      // 处理小数位显示
      this.scoreDiffList.forEach((item) => {
        if (item.currentScore !== "-" && item.currentScore.endsWith(".00")) {
          item.currentScore = parseInt(item.currentScore).toString();
        }
        if (item.kpiScore !== "-" && item.kpiScore.endsWith(".00")) {
          item.kpiScore = parseInt(item.kpiScore).toString();
        }
      });

      if (this.scoreDiffList.length > 0) {
        this.scoreDiffVisible = true;
      }
    },
    // 判断发起年终确认按钮是否禁用
    isInitiateConfirmDisabled(row) {
      if (!row) return true;

      // 1. 检查4个季度的分数是否都为空，如果都为空则禁用
      // 只检查 row 对象上的季度分数，不再回退检查 kpiGroups
      const allScoresMissing =
        !this.isValidNumber(row.firstQuarter) &&
        !this.isValidNumber(row.secondQuarter) &&
        !this.isValidNumber(row.thirdQuarter) &&
        !this.isValidNumber(row.fourQuarter);

      if (allScoresMissing) {
        return true;
      }

      // 2. 保留原有逻辑
      // examineStatus 和 finalCheck 都等于 null 时，可以发起年终绩效
      if (row.examineStatus === null && row.finalCheck === null) {
        return false;
      }

      // examineStatus 为 null，finalCheck 不为空时，发起年终绩效按钮置灰
      if (row.examineStatus === null && row.finalCheck !== null) {
        return true;
      }

      // examineStatus 为 pass 或 under_review 时，发起年终绩效按钮置灰
      if (
        row.examineStatus === "pass" ||
        row.examineStatus === "under_review"
      ) {
        return true;
      }

      return false;
    },
  },
};
</script>

<style lang="scss" scoped>
.staff-annual-assessment {
  padding: 20px;
  background-color: white;
  height: 100%;

  .search-top {
    .search-item {
      display: inline-block;
      margin-right: 15px;
      margin-bottom: 10px;
    }
  }

  .content {
    ::v-deep .dytable-view-container {
      padding: 0;
    }
  }
}
</style>
