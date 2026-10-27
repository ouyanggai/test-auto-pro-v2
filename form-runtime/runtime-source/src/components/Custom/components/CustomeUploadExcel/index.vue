<!--
 * @Descripttion: excel 第一张表数据读取模板
 * @Author: hejing
 * @Date: 2025-06-18
-->
<template>
  <div>
    <el-upload
      ref="upload"
      action="#"
      accept=".xlsx,.xls"
      :limit="1"
      :file-list="fileList"
      :show-file-list="false"
      :http-request="uploadHttpRequest"
    >
      <el-button type="primary" :disabled="disabled">{{ placeholder || '导入Excel' }}</el-button>
    </el-upload>

    <!-- 数据预览弹窗 -->
    <el-dialog
      title="Excel数据预览"
      :visible.sync="dialogVisible"
      width="1000px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
      append-to-body
    >
      <div class="dialog-content">
        <div class="info-section">
          <!-- 新增：错误提示 -->
          <el-alert
            v-if="hasValidationErrors"
            :title="`导入${excelData.length}条数据，存在数据格式有误，请检查并修正后重新导入`"
            type="error"
            :closable="false"
            show-icon
            style="margin-bottom: 20px;"
          />
          <el-alert
            v-else
            :title="`成功导入Excel文件，共${excelData.length}行数据`"
            type="success"
            :closable="false"
            show-icon
            style="margin-bottom: 20px;"
          />
        </div>

        <div class="table-section">
          <el-table
            :data="excelData"
            border
            stripe
            max-height="500"
            style="width: 100%"
            v-loading="tableLoading"
          >
            <el-table-column
              v-for="(header, index) in headers"
              :key="index"
              :prop="header"
              :min-width="120"
              show-overflow-tooltip
            >
              <template slot="header">
                <span v-if="isHeaderRequired(header)" style="color:#f56c6c">*</span>{{ header }}
              </template>
              <template slot-scope="scope">
                <span :class="{ 'error-cell': scope.row[`${header}_error`] }">
                  {{ formatCellValue(scope.row[header]) }}
                </span>
                <div v-if="scope.row[`${header}_error`]" class="error-tip">
                  {{ scope.row[`${header}_error`] }}
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>

      <div slot="footer" class="dialog-footer">
        <el-button @click="handleCloseDialog">取消</el-button>
        <el-button type="primary" @click="handleConfirmData" :disabled="hasValidationErrors">确认导入</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import XLSX from '@/lib/xlsx/xlsx.full.min.js';

/* eslint-disable */
export default {
  name: 'CustomeUploadExcel',
  components: {},
  props: {
    value: {
      type: String,
      default: ''
      // value 示例
      // JSON.stringify({
      //   日期: [
      //     {
      //       required: true,
      //       message: '请输入日期',
      //       trigger: 'blur'
      //     },
      //     {
      //       type: 'data',
      //       message: '请输入日期格式',
      //       trigger: 'blur'
      //     }
      //   ],
      //   类别: [
      //     {
      //       required: false,
      //       message: '请输入类别',
      //       trigger: 'blur'
      //     },
      //     {
      //       max: 50,
      //       message: '字符不超过50',
      //       trigger: 'blur'
      //     }
      //   ],
      //   明细: [
      //     {
      //       required: false,
      //       message: '请输入明细',
      //       trigger: 'blur'
      //     },
      //     {
      //       max: 50,
      //       message: '字符不超过50',
      //       trigger: 'blur'
      //     }
      //   ],
      //   单价: [
      //     {
      //       required: true,
      //       message: '请输入单价',
      //       trigger: 'blur'
      //     },
      //     {
      //       type: 'number',
      //       message: '请输入数字类型',
      //       trigger: 'blur'
      //     },
      //     {
      //       max: 50,
      //       message: '字符不超过50',
      //       trigger: 'blur'
      //     }
      //   ],
      //   数量: [
      //     {
      //       required: true,
      //       message: '请输入数量',
      //       trigger: 'blur'
      //     },
      //     {
      //       type: 'number',
      //       message: '请输入数字类型',
      //       trigger: 'blur'
      //     },
      //     {
      //       max: 50,
      //       message: '字符不超过50',
      //       trigger: 'blur'
      //     }
      //   ],
      // })
    },
    placeholder: {
      type: String,
      default: ''
    },
    disabled: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      fileList:[],
      excelData: [], // 存储解析后的Excel数据
      headers: [], // 存储表头
      dialogVisible: false, // 弹窗显示状态
      tableLoading: false, // 表格加载状态
      hasValidationErrors: false, // 是否有校验错误
    };
  },
  created() {

  },
  mounted() { },
  computed: {
    // 传递进来 期望的表头
    expectedHeaders() {
      console.log(this.value, 'this.value');
      try {
        // 如果 value 是字符串，尝试解析它
        const rules = typeof this.value === 'string' ? JSON.parse(this.value) : this.value;
        return rules && typeof rules === 'object' ? Object.keys(rules) : [];
      } catch (error) {
        console.error('解析 value 失败:', error);
        return [];
      }
    }
  },
  methods: {
    uploadHttpRequest(param) {
      const file = param.file;

      // 检查文件类型
      if (!file.name.match(/\.(xlsx|xls)$/i)) {
        this.$message.error('请上传Excel文件（.xlsx或.xls格式）');
        return;
      }

      // 重置状态
      this.excelData = [];
      this.headers = [];
      this.hasValidationErrors = false;
      this.dialogVisible = false;

      // 重置el-upload组件
      this.$nextTick(() => {
        if (this.$refs.upload) {
          this.$refs.upload.clearFiles();
        }
      });

      this.tableLoading = true;
      const reader = new FileReader();
      reader.onload = (e) => {
        try {
          const data = e.target.result;
          const workbook = XLSX.read(data, { type: 'binary' });

          // 获取第一个工作表
          const firstSheetName = workbook.SheetNames[0];
          const worksheet = workbook.Sheets[firstSheetName];

          // 将工作表转换为JSON数据
          const jsonData = XLSX.utils.sheet_to_json(worksheet, {
            header: 1, // 使用数组格式，包含表头
            defval: '', // 空单元格的默认值
            raw: false, // 启用原始值解析，包括日期
            dateNF: 'yyyy-mm-dd' // 日期格式
          });

          // 处理数据：第一行作为表头，其余行作为数据
          if (jsonData.length > 0) {
            console.log(jsonData[0], 'jsonData[0]')
            this.headers = jsonData[0].map(item=>item);
            let dataRows = jsonData.slice(1);
            console.log(dataRows, 'dataRows')
            console.log(this.headers, 'this.headers')
            console.log(this.expectedHeaders, 'this.expectedHeaders')

            // 校验表头是否一致expectedHeaders  headers
            if (this.expectedHeaders.length > 0) {
              const headerErrors = [];
              this.expectedHeaders.forEach((expectedHeader, index) => {
                if (this.headers[index] !== expectedHeader) {
                  headerErrors.push(`第${index + 1}列表头应为"${expectedHeader}"，实际为"${this.headers[index]}"`);
                }
              });

              if (headerErrors.length > 0) {
                // this.$message.error(`表头格式不正确：${headerErrors.join('；')}，请使用正确的模板文件`);
                this.$message.error(`请使用正确的模板文件，请检查表头是否一致。`);
                this.tableLoading = false;
                return;
              }
            }

            // ===== 整行空行过滤代码 =====
            dataRows = dataRows.filter(row => {
              // 检查整行是否为空：所有单元格都为空值
              const isEmptyRow = !row.some(cell =>
                cell !== "" && cell !== null && cell !== undefined
              );
              return !isEmptyRow; // 保留非空行
            });

            // 将数据转换为对象数组格式
            const processedData = dataRows.map(row => {
              const obj = {};
              this.headers.forEach((header, index) => {
                let cellValue = row[index] || '';
                obj[header] = cellValue;
              });
              return obj;
            });

            this.excelData = processedData;
            console.log(this.excelData,'this.excelData')

            // 显示弹窗
            this.dialogVisible = true;

            // 执行数据校验
            this.validateData();

            console.log('解析的Excel数据:', {
              headers: this.headers,
              data: processedData
            });
          } else {
            this.$message.warning('解析Excel文件失败，请使用正确的模板文件');
          }
        } catch (error) {
          console.error('解析Excel文件失败:', error);
          this.$message.error('解析Excel文件失败，请使用正确的模板文件');
        } finally {
          this.tableLoading = false;
        }
      };

      reader.onerror = () => {
        this.$message.error('读取文件失败，请使用正确的模板文件');
        this.tableLoading = false;
      };

      reader.readAsBinaryString(file);
    },

    // 关闭弹窗
    handleCloseDialog() {
      this.dialogVisible = false;
      this.excelData = [];
      this.headers = [];
      this.hasValidationErrors = false;
      this.tableLoading = false;
      this.fileList = [];

      // 强制重置el-upload组件状态
      this.$nextTick(() => {
        if (this.$refs.upload) {
          this.$refs.upload.clearFiles();
        }
      });
    },

    // 确认导入数据
    handleConfirmData() {
      // 1. 解析规则，找出所有 type: 'date' 或 'data' 的字段
      let rules;
      console.log(this.value, 'this.value')
      try {
        rules = typeof this.value === 'string' ? JSON.parse(this.value) : this.value;
      } catch (error) {
        rules = {};
      }
      const dateFields = Object.keys(rules).filter(key =>
        (rules[key] || []).some(rule => rule.type && (rule.type === 'date' || rule.type === 'data'))
      );
      console.log(dateFields,'dateFields')
      // 2. 移除 *_error 字段，并格式化日期
      const cleanData = this.excelData.map(row => {
        const newRow = { ...row };
        Object.keys(newRow).forEach(key => {
          if (key.endsWith('_error')) {
            delete newRow[key];
          }
          // 格式化日期字段
          if (dateFields.includes(key) && newRow[key]) {
            console.log(newRow[key],'newRow[key]')
            newRow[key] = this.normalizeDate(newRow[key]);
            console.log(newRow[key],'newRow[key]')
          }
        });
        return newRow;
      });
      this.$emit('input', JSON.stringify(cleanData));
      // this.$message({
      //   message: `成功导入Excel文件，共${this.excelData.length}行数据`,
      //   type: 'success',
      //   zIndex: 3000
      // });
      this.handleCloseDialog();
    },
    // 格式化单元格值
    formatCellValue(value) {
      if (value === null || value === undefined || value === '') {
        return '-';
      }

      // 如果是日期，统一格式化为 YYYY-MM-DD
      if (this.isValidDate(value)) {
        return this.normalizeDate(value);
      }

      return String(value);
    },

    // 统一日期格式为 YYYY-MM-DD
    normalizeDate(value) {
      const dateStr = value.toString().trim();

      // 解析日期
      let year, month, day;

      if (/^\d{4}-\d{1,2}-\d{1,2}$/.test(dateStr)) {
        // YYYY-MM-DD 格式
        const parts = dateStr.split('-');
        year = parseInt(parts[0]);
        month = parseInt(parts[1]);
        day = parseInt(parts[2]);
      } else if (/^\d{4}\/\d{1,2}\/\d{1,2}$/.test(dateStr)) {
        // YYYY/MM/DD 格式
        const parts = dateStr.split('/');
        year = parseInt(parts[0]);
        month = parseInt(parts[1]);
        day = parseInt(parts[2]);
      } else if (/^\d{4}\.\d{1,2}\.\d{1,2}$/.test(dateStr)) {
        // YYYY.MM.DD 格式
        const parts = dateStr.split('.');
        year = parseInt(parts[0]);
        month = parseInt(parts[1]);
        day = parseInt(parts[2]);
      } else if (/^\d{4}年\d{1,2}月\d{1,2}日$/.test(dateStr)) {
        // YYYY年MM月DD日 格式
        const match = dateStr.match(/^(\d{4})年(\d{1,2})月(\d{1,2})日$/);
        year = parseInt(match[1]);
        month = parseInt(match[2]);
        day = parseInt(match[3]);
      }

      // 格式化为 YYYY-MM-DD
      return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
    },

    // 数据校验
    validateData() {
      this.hasValidationErrors = false;
      let rules;
      try {
        rules = typeof this.value === 'string' ? JSON.parse(this.value) : this.value;
      } catch (error) {
        console.error('解析验证规则失败:', error);
        return;
      }

      if (!rules || typeof rules !== 'object') {
        // No rules, no validation
        return;
      }

      console.log(this.expectedHeaders,'this.expectedHeaders');

      // 校验数据格式 - 一次校验全部字段
      this.excelData.forEach((row, rowIndex) => {
        this.expectedHeaders.forEach(header => {
          // 清除旧的错误标记
          this.$set(row, `${header}_error`, undefined);

          const cellValue = row[header];
          const columnRules = rules[header];

          if (columnRules && Array.isArray(columnRules)) {
            // 遍历该字段的所有校验规则
            for (let i = 0; i < columnRules.length; i++) {
              const rule = columnRules[i];

              // 1. required 校验
              if (rule.required) {
                console.log('校验required:', header, cellValue, rule.message);
                if (cellValue === null || cellValue === undefined || cellValue.toString().trim() === '') {
                  this.$set(row, `${header}_error`, rule.message);
                  this.hasValidationErrors = true;
                }
              } else {
                // required 为 false 时，如果值为空则跳过后续校验
                if (cellValue === null || cellValue === undefined || cellValue.toString().trim() === '') {
                  continue; // 跳过当前规则，继续下一个规则
                }
              }

              // 2. type 校验（只有当有值时才校验）
              if (rule.type && cellValue !== null && cellValue !== undefined && cellValue.toString().trim() !== '') {
                switch (rule.type.toLowerCase()) {
                  case 'date':
                  case 'data': // 兼容 'data'
                    if (!this.isValidDate(cellValue)) {
                      this.$set(row, `${header}_error`, rule.message);
                      this.hasValidationErrors = true;
                      break;
                    }
                    break;
                  case 'number':
                    if (!this.isValidNumber(cellValue)) {
                      this.$set(row, `${header}_error`, rule.message);
                      this.hasValidationErrors = true;
                      break;
                    }
                }
              }

              // 3. min/max 字符长度校验（只有当有值时才校验）
              if (rule.hasOwnProperty('max') && cellValue && cellValue.toString().length > rule.max) {
                this.$set(row, `${header}_error`, rule.message);
                this.hasValidationErrors = true;
                break;
              }

              if (rule.hasOwnProperty('min') && cellValue && cellValue.toString().length < rule.min) {
                this.$set(row, `${header}_error`, rule.message);
                this.hasValidationErrors = true;
                break;
              }
            }
          }
        });
      });

      // if (this.hasValidationErrors) {
      //   this.$message({
      //     message: '数据格式有误，请检查并修正后重新导入',
      //     type: 'warning',
      //     duration: 0
      //   });
      // }
    },

    // 校验日期格式
    isValidDate(value) {
      if (!value || value.toString().trim() === '') {
        return false;
      }

      const dateStr = value.toString().trim();

      // 支持的日期格式正则表达式
      const datePatterns = [
        /^\d{4}-\d{1,2}-\d{1,2}$/,           // YYYY-MM-DD 或 YYYY-M-D
        /^\d{4}\/\d{1,2}\/\d{1,2}$/,         // YYYY/MM/DD 或 YYYY/M/D
        /^\d{4}\.\d{1,2}\.\d{1,2}$/,         // YYYY.MM.DD 或 YYYY.M.D
        /^\d{4}年\d{1,2}月\d{1,2}日$/,        // YYYY年MM月DD日
      ];

      // 检查是否匹配任一格式
      const matchedPattern = datePatterns.find(pattern => pattern.test(dateStr));
      if (!matchedPattern) {
        return false;
      }

      // 解析日期
      let year, month, day;

      if (/^\d{4}-\d{1,2}-\d{1,2}$/.test(dateStr)) {
        // YYYY-MM-DD 格式
        const parts = dateStr.split('-');
        year = parseInt(parts[0]);
        month = parseInt(parts[1]);
        day = parseInt(parts[2]);
      } else if (/^\d{4}\/\d{1,2}\/\d{1,2}$/.test(dateStr)) {
        // YYYY/MM/DD 格式
        const parts = dateStr.split('/');
        year = parseInt(parts[0]);
        month = parseInt(parts[1]);
        day = parseInt(parts[2]);
      } else if (/^\d{1,2}\/\d{1,2}\/\d{4}$/.test(dateStr)) {
        // MM/DD/YYYY 格式
        const parts = dateStr.split('/');
        month = parseInt(parts[0]);
        day = parseInt(parts[1]);
        year = parseInt(parts[2]);
      } else if (/^\d{1,2}-\d{1,2}-\d{4}$/.test(dateStr)) {
        // MM-DD-YYYY 格式
        const parts = dateStr.split('-');
        month = parseInt(parts[0]);
        day = parseInt(parts[1]);
        year = parseInt(parts[2]);
      } else if (/^\d{4}\.\d{1,2}\.\d{1,2}$/.test(dateStr)) {
        // YYYY.MM.DD 格式
        const parts = dateStr.split('.');
        year = parseInt(parts[0]);
        month = parseInt(parts[1]);
        day = parseInt(parts[2]);
      } else if (/^\d{1,2}\.\d{1,2}\.\d{4}$/.test(dateStr)) {
        // MM.DD.YYYY 格式
        const parts = dateStr.split('.');
        month = parseInt(parts[0]);
        day = parseInt(parts[1]);
        year = parseInt(parts[2]);
      } else if (/^\d{4}年\d{1,2}月\d{1,2}日$/.test(dateStr)) {
        // YYYY年MM月DD日 格式
        const match = dateStr.match(/^(\d{4})年(\d{1,2})月(\d{1,2})日$/);
        year = parseInt(match[1]);
        month = parseInt(match[2]);
        day = parseInt(match[3]);
      } else if (/^\d{1,2}月\d{1,2}日\d{4}年$/.test(dateStr)) {
        // MM月DD日YYYY年 格式
        const match = dateStr.match(/^(\d{1,2})月(\d{1,2})日(\d{4})年$/);
        month = parseInt(match[1]);
        day = parseInt(match[2]);
        year = parseInt(match[3]);
      } else if (/^\d{4}\d{2}\d{2}$/.test(dateStr)) {
        // YYYYMMDD 格式
        year = parseInt(dateStr.substring(0, 4));
        month = parseInt(dateStr.substring(4, 6));
        day = parseInt(dateStr.substring(6, 8));
      }

      // 验证日期有效性
      if (year < 1900 || year > 2200) {
        return false;
      }

      if (month < 1 || month > 12) {
        return false;
      }

      const date = new Date(year, month - 1, day);
      return date.getFullYear() === year &&
             date.getMonth() === month - 1 &&
             date.getDate() === day;
    },

    // 校验数字格式
    isValidNumber(value) {
      if (value === null || value === undefined || value === '') {
        return false;
      }

      const num = parseFloat(value);
      if (isNaN(num)) {
        return false;
      }

      // Check if the number is within the safe integer range
      if (num > Number.MAX_SAFE_INTEGER || num < Number.MIN_SAFE_INTEGER) {
        return false;
      }

      return true;
    },
    isHeaderRequired(header) {
      let rules = typeof this.value === 'string' ? JSON.parse(this.value) : this.value;
      console.log(rules,'rules')
      const columnRules = rules[header] || [];
      return columnRules.some(rule => rule.required === true);
    },
  },
};
</script>
<style lang="scss" scoped>
.dialog-content {
  .info-section {
    margin-bottom: 20px;

    .file-info {
      display: flex;
      gap: 20px;
      margin-top: 10px;

      span {
        color: #606266;
        font-size: 14px;
      }
    }
  }

  .table-section {
    .el-table {
      .el-table__header-wrapper {
        th {
          background-color: #f5f7fa;
          color: #606266;
          font-weight: 600;
        }
      }
    }
  }
}

.dialog-footer {
  text-align: right;
}

.error-cell {
  color: #f56c6c;
  background-color: #fef0f0;
}

.error-tip {
  color: #f56c6c;
  font-size: 12px;
  margin-top: 2px;
}
::v-deep .el-dialog__body{
  padding: 20px !important;
}

// .file-wrap {
//   margin:0 15px;
//   color:#1989FA;
//   padding: 4px 20px;
//   .file-icon {
//     color:#409eff;
//     cursor: pointer;
//     margin-left: 16px;
//   }
// }
// .no-allow {
//   background-color: #f5f7fa;
//   border-color: #e4e7ed;
//   color: #c0c4cc;
//   cursor: not-allowed;
//   .file-icon {
//     color: #c0c4cc;
//   }
// }

/* 可以放在全局样式或当前组件的 <style> 标签里（去掉 scoped） */
.el-message {
  z-index: 3000 !important;
}
</style>
