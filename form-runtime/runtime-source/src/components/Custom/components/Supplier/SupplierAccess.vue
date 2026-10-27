<!--
 * @Descripttion: 供应商年度综合评估
 * @Author: junshao
-->
<template>
  <div class="supplier-access" :style="rootStyle">
    <!-- 页面主体：左侧供应商评估列表 + 右侧表单信息 -->
    <div class="page-body">
      <!-- 左侧：供应商评估列表，点击切换右侧表单 -->
      <div class="supplier-aside">
        <div class="aside-title">
          <p>供应商评估列表（{{ supplierList.length }}）</p>
          <p v-if="!disabled" class="aside-tip">单次评估最多可评估10个供应商</p>
        </div>
        <div v-if="!disabled" class="aside-toolbar">
          <el-button type="primary" size="small" icon="el-icon-search" class="add-supplier-btn"
            data-testid="1877c89f-b1b9-44f3-817e-a17a96443551" @click="openSupplierDialog">选择供应商</el-button>
        </div>
        <div class="aside-list">
          <div v-for="(item, index) in supplierList" :key="index" class="aside-item" :class="{ active: index === activeIndex }"
            @click="handleSelectSupplier(index)">
            <span class="aside-index">{{ index + 1 }}</span>
            <span class="aside-name" :title="item.supplierName">{{ item.supplierName || '-' }}</span>
            <i v-if="!disabled" class="el-icon-delete remove-icon" @click.stop="handleRemoveSupplier(index)"></i>
          </div>
          <!-- 空状态 -->
          <div v-if="!supplierList.length" class="aside-empty">暂无数据</div>
        </div>
      </div>

      <!-- 右侧：所选供应商的年度评估表单 -->
      <div class="supplier-form">
        <el-form v-if="activeSupplier" :model="activeSupplier" label-position="right" label-width="140px" size="small">
          <div class="form-row">
            <div class="form-item">
              <el-form-item label="供应商名称" prop="supplierName" :rules="[{ required: true, message: '请选择供应商', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.supplierName" data-testid="6beb1e91-9589-47c5-a787-252aea3c54e4" disabled placeholder="请输入供应商名称"></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="归属公司" prop="companyId" :rules="[{ required: true, message: '请选择归属公司', trigger: 'change' }]">
                <el-select v-model="activeSupplier.companyId" data-testid="53826931-d918-46dd-aa7c-3e536ee4234e" disabled placeholder="请选择" class="form-control">
                  <el-option v-for="opt in companyOptions" :key="opt.value" :label="opt.label" :value="opt.value"></el-option>
                </el-select>
              </el-form-item>
            </div>
          </div>
          <div class="form-row">
            <div class="form-item">
              <el-form-item label="合作类型" prop="cooperationType" :rules="[{ required: true, message: '请选择合作类型', trigger: 'change' }]">
                <el-select v-model="activeSupplier.cooperationType" data-testid="42d8efe4-c78e-43d3-887c-3baf9d17b223" disabled placeholder="请选择合作类型" class="form-control">
                  <el-option v-for="opt in cooperationTypeOptions" :key="opt" :label="opt" :value="opt"></el-option>
                </el-select>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="入库时间" prop="storageDate" :rules="[{ required: true, message: '请选择入库时间', trigger: 'change' }]">
                <el-date-picker v-model="activeSupplier.storageDate" data-testid="ecaaa8fd-eba8-44c0-b839-15d379827937" disabled type="date" value-format="yyyy-MM-dd"
                  placeholder="请选择入库时间" class="form-control"></el-date-picker>
              </el-form-item>
            </div>
          </div>
          <div class="form-row">
            <div class="form-item">
              <el-form-item label="首次合作起始日期" prop="firstCooperationDate" :rules="[{ required: true, message: '请选择首次合作起始日期', trigger: 'change' }]">
                <el-date-picker v-model="activeSupplier.firstCooperationDate" data-testid="1d14bd9e-59f1-47a6-b076-6f3204aad47f" :disabled="disabled || activeSupplier.firstCooperationDateLocked" type="date" value-format="yyyy-MM-dd"
                  placeholder="请选择日期" class="form-control"></el-date-picker>
              </el-form-item>
            </div>
          </div>
          <div class="form-row">
            <div class="form-item">
              <el-form-item label="评估级别" prop="evalLevel" :rules="[{ required: true, message: '请选择评估级别', trigger: 'change' }]">
                <el-select v-model="activeSupplier.evalLevel" data-testid="c1f12a00-31fa-41e8-aa07-87e413330d1a" :disabled="disabled" placeholder="请选择" class="form-control">
                  <el-option v-for="opt in assessLevelOptions" :key="opt" :label="opt" :value="opt"></el-option>
                </el-select>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="评估时间" prop="evalTime" :rules="[{ required: true, message: '请选择评估时间', trigger: 'change' }]">
                <el-date-picker v-model="activeSupplier.evalTime" data-testid="a6ad2c7e-fe2c-4f91-9256-5d97a58b3e96" :disabled="disabled" type="date" value-format="yyyy-MM-dd"
                  :picker-options="evalTimePickerOptions" placeholder="请选择日期" class="form-control"></el-date-picker>
              </el-form-item>
            </div>
          </div>
          <div class="form-row">
            <div class="form-item">
              <el-form-item label="经营范围" prop="businessScope" :rules="[{ required: true, message: '请输入经营范围', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.businessScope" data-testid="11875233-94af-444a-9931-a78ee8b40460" disabled type="textarea" :autosize="{ minRows: 2}" placeholder="请输入经营范围"></el-input>
              </el-form-item>
            </div>
          </div>
          <div class="form-row">
            <div class="form-item">
              <el-form-item label="近期履约情况" prop="recentPerformance" :rules="[{ required: true, message: '请填写近期履约情况', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.recentPerformance" data-testid="65730624-7444-4168-8d6e-32c30a4fc9d1" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="近期履约情况，包括质量、安全、进度、成本、服务、合规等"></el-input>
              </el-form-item>
            </div>
          </div>
          <div class="form-row">
            <div class="form-item">
              <el-form-item label="异常情况记录" prop="exceptionRecord">
                <el-input v-model="activeSupplier.exceptionRecord" data-testid="efe10e65-ca9a-49f5-a42c-4c84ee7353df" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
          </div>
          <!-- 附件：年度评估专属附件，存表单 attachments 字段（不区分类型） -->
          <div class="form-row">
            <div class="form-item">
              <el-form-item label="附件" prop="attachments" :rules="[{ required: true, message: '请上传附件', trigger: 'change' }]">
                <eleUpload :attachFile="activeSupplier.attachments" data-testid="f3c6f0c1-f4db-44a2-acf1-25de85acb17e" :isDuplicateOverwrite="false" :size="300" :uploadLimit="20" :showOnly="disabled" @change="handleAttachmentsChange"></eleUpload>
              </el-form-item>
            </div>
          </div>
          <div class="form-row">
            <div class="form-item">
              <el-form-item label="备注" prop="remark">
                <el-input v-model="activeSupplier.remark" data-testid="cd87d563-8d73-4523-ad24-30b9663edf56" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="备注信息"></el-input>
              </el-form-item>
            </div>
          </div>
        </el-form>
        <!-- 无数据时的占位 -->
        <div v-else class="form-empty">请先点击左侧「选择供应商」添加数据</div>
      </div>
    </div>

    <!-- 选择供应商弹窗：当前公司供应商台账数据块（一行两个），点击切换选中；已在评估列表中的默认选中 -->
    <el-dialog title="选择供应商" :visible.sync="supplierDialogVisible" width="720px" :close-on-click-modal="false" append-to-body @close="handleDialogClosed">
      <div class="selected-count">已选择 {{ selectedLedgerIds.length }} 家供应商</div>
      <div style="margin-bottom: 10px;">
        <el-input v-model.trim="searchSupplier" data-testid="4bf7d8aa-e661-4f3d-b141-b647a9b86d82" placeholder="请输入供应商名称或编码/按回车搜索" :validate-event="false" @keyup.enter.native="loadLedgerList"></el-input>
      </div>
      <div class="supplier-grid">
        <div
          v-for="item in ledgerList"
          :key="item.id"
          :class="['supplier-block', { selected: selectedLedgerIds.indexOf(item.id) > -1 }]"
          data-testid="e0a9231c-f8d1-43ff-b4d7-8a7d844deaff"
          @click="toggleLedger(item)"
        >
          <div class="block-name" :title="item.supplierName">{{ item.supplierName || '-' }}</div>
          <div class="block-code">{{ item.supplierCode || '-' }}</div>
          <i v-if="selectedLedgerIds.indexOf(item.id) > -1" class="el-icon-check block-check-icon"></i>
        </div>
        <!-- 空状态 -->
        <div v-if="!ledgerList.length" class="supplier-empty">当前公司暂无供应商台账数据</div>
      </div>
      <template slot="footer">
        <el-button size="small" @click="supplierDialogVisible = false">取 消</el-button>
        <el-button type="primary" size="small" data-testid="fdc6ad8f-0bbb-4d72-8bd6-cfc8a0ddc050" @click="handleConfirmSelect">确 定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import Api from '@/api';
import eleUpload from '@/components/EleUpload';
import { localstorageGet } from '@/utils/auth';

// 解析 form-making 传入的 value（JSON 字符串 -> 供应商评估数组），空值/异常兜底为空数组
function safeParseList(val) {
  if (!val) return [];
  try {
    const parsed = JSON.parse(val);
    return Array.isArray(parsed) ? parsed : [];
  } catch (error) {
    return [];
  }
}

export default {
  name: 'SupplierAccess',
  components: { eleUpload },
  props: {
    // form-making v-model 通道：JSON.stringify(supplierList)，空列表为空字符串
    value: {
      type: String,
      default: ''
    },
    // 审批态：控件全部禁用，隐藏添加/删除入口
    disabled: {
      type: Boolean,
      default: false
    },
    // 打印态（form-making 注入，暂无特殊渲染）
    printRead: {
      type: Boolean,
      default: false
    },
    // form-making 设计器配置的 options.width：组件整体宽度，未配置时撑满容器
    width: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      searchSupplier: '',
      // 页面展示的供应商评估数据（字段模型见 createEvalRecord）
      supplierList: safeParseList(this.value),
      // 左侧列表当前选中项下标
      activeIndex: 0,
      // 合作类型下拉选项（静态，后续由接口提供）
      cooperationTypeOptions: ['供应商', '施工商', '服务商', '其他类'],
      // 评估级别下拉选项（静态）
      assessLevelOptions: ['A级', 'B级', 'C级', 'D级', '黑名单'],
      // 选择供应商弹窗显示状态
      supplierDialogVisible: false,
      // 当前公司供应商台账列表（弹窗数据块）
      ledgerList: [],
      // 弹窗中已勾选的台账 ID 集合
      selectedLedgerIds: [],
      // 审批/打印态归属公司反显用：全量公司列表（发起态不加载）
      allCompanyList: []
    };
  },
  computed: {
    // 组件整体宽度：应用设计器 options.width（如 '800px'/'50%'），空值不设、走样式表的 width: 100%
    rootStyle() {
      return this.width ? { width: this.width } : null;
    },
    // 归属公司选项：审批/打印态换全量公司列表（按表单数据持久化的 companyId 匹配名称，
    // 反显结果与查看者当前所选公司无关）；发起态/列表未就绪或加载失败时维持现状取当前用户所选公司
    companyOptions() {
      if ((this.disabled || this.printRead) && this.allCompanyList.length) {
        // String() 对齐类型：持久化数据里的 companyId 是字符串（localStorage 直取 + JSON 往返），
        // 后端返回的 id 若是数值，el-select 的 === 匹配会失配、仍会裸显 id
        return this.allCompanyList.map(c => ({ label: c.name, value: String(c.id) }));
      }
      return [
        { label: localstorageGet('companyName'), value: localstorageGet('companyId') }
      ];
    },
    // 当前选中的供应商（右侧表单直接绑定其字段）
    activeSupplier() {
      return this.supplierList[this.activeIndex] || null;
    },
    // 评估时间可选范围：入库日期（不含）~ 当前系统日期（含）
    evalTimePickerOptions() {
      let minTime = null;
      const storageDate = this.activeSupplier ? this.activeSupplier.storageDate : '';
      const parts = String(storageDate || '').slice(0, 10).split('-').map(Number);
      if (parts.length === 3 && parts.every(n => !isNaN(n))) {
        minTime = new Date(parts[0], parts[1] - 1, parts[2]).getTime();
      }
      return {
        disabledDate: (time) => {
          // 入库日期当天及之前不可选（不含入库日期）
          if (minTime !== null && time.getTime() <= minTime) return true;
          // 当前系统日期之后不可选（当天可选）
          return time.getTime() > Date.now();
        }
      };
    }
  },
  watch: {
    // 外部值变化（如 setData 回显、流程草稿恢复）时重新装载；
    // 与当前数据一致时跳过，避免组件自身 emit 引发的回环覆盖
    value(val) {
      const parsed = safeParseList(val);
      if (JSON.stringify(parsed) === JSON.stringify(this.supplierList)) return;
      this.supplierList = parsed;
      this.activeIndex = Math.min(this.activeIndex, Math.max(parsed.length - 1, 0));
    },
    // 数据变化同步回 form-making；空列表发空字符串而非 '[]'，
    // 保证设计器配置的"必填"校验能真正拦截空数据
    supplierList: {
      handler(list) {
        const payload = list.length ? JSON.stringify(list) : '';
        if (payload !== this.value) {
          this.$emit('input', payload);
        }
      },
      deep: true
    }
  },
  created() {
    this.loadLedgerList();
    // 审批/打印态归属公司反显数据源
    this.loadCompanyListForView();
  },
  mounted() { },
  methods: {
    // 查询当前用户所属公司的供应商台账（弹窗数据块来源）
    loadLedgerList() {
      // 审批态弹窗不可达，无需拉取
      if (this.disabled) return;
      this.$axios.post(
        Api.supplierCenter.getSupplierListByNameOrCode,
        { data: { keyword: this.searchSupplier, companyId: localstorageGet('companyId'), ledgerStatus: 1, limit: 9999 }},
        (res) => {
          if (!res.isSuccess) return;
          this.ledgerList = res.data || [];
        }
      );
    },

    // 审批/打印态反显用：拉全量公司列表做归属公司选项来源；发起态不请求，维持现状
    loadCompanyListForView() {
      if (!this.disabled && !this.printRead) return;
      this.$axios.post(
        Api.user.getAllCompanyList,
        { data: {}},
        (res) => {
          if (!res.isSuccess) return;
          this.allCompanyList = res.data || [];
        }
      );
    },

    // 台账是否已添加进评估列表（按 ledgerId 判断，弹窗中置灰不可再选）
    isAddedLedger(item) {
      return this.supplierList.some(s => s.ledgerId === item.id);
    },

    // 打开选择供应商弹窗
    openSupplierDialog() {
      // 已在评估列表中的供应商默认选中（过滤掉台账中已不存在的脏数据）
      this.selectedLedgerIds = this.supplierList
        .map(s => s.ledgerId)
        .filter(id => this.ledgerList.some(item => item.id === id));
      this.supplierDialogVisible = true;
    },

    // 弹窗数据块点击：切换选中态；新增选中数（扣除已在列表中的）受单次评估上限（10 家）约束
    toggleLedger(item) {
      const index = this.selectedLedgerIds.indexOf(item.id);
      if (index > -1) {
        this.selectedLedgerIds.splice(index, 1);
        return;
      }
      this.selectedLedgerIds.push(item.id);
      // 仅统计本次将新加入评估列表的选中数，已在列表中的不计
      const newSelectedCount = this.selectedLedgerIds.filter(id => !this.supplierList.some(s => s.ledgerId === id)).length;
      if (this.supplierList.length + newSelectedCount > 10) {
        this.selectedLedgerIds.splice(this.selectedLedgerIds.indexOf(item.id), 1);
        this.$message.warning('单次评估最多可评估10个供应商');
      }
    },

    // 弹窗确定：按最终勾选状态同步评估列表——
    // 被取消勾选的已添加供应商从列表移除；新勾选的台账转为评估记录追加，并选中第一新增项
    handleConfirmSelect() {
      // 移除被取消勾选的（勾选后又取消再勾回的不受影响，数据保留）
      this.supplierList = this.supplierList.filter(
        s => this.selectedLedgerIds.indexOf(s.ledgerId) > -1
      );
      // 追加新勾选中尚未在列表里的
      const selectedList = this.ledgerList.filter(item =>
        this.selectedLedgerIds.indexOf(item.id) > -1 && !this.isAddedLedger(item)
      );
      const startIndex = this.supplierList.length;
      if (selectedList.length) {
        this.supplierList = this.supplierList.concat(selectedList.map(item => this.createEvalRecord(item)));
        this.activeIndex = startIndex;
      } else {
        // 无新增时保持原选中行，列表缩短时收敛到有效范围
        this.activeIndex = Math.min(this.activeIndex, Math.max(this.supplierList.length - 1, 0));
      }
      this.selectedLedgerIds = [];
      this.supplierDialogVisible = false;
    },

    // 弹窗关闭（确定/取消/右上角 X 均触发）：重置搜索词并按空关键词重新查询，
    // 保证下次打开是全量供应商列表（已添加供应商的回显预选中依赖全量数据）
    handleDialogClosed() {
      this.searchSupplier = '';
      this.loadLedgerList();
    },

    // 台账数据 -> 评估记录：带入基础信息，评估时间取当前系统日期，其余评估字段待填
    createEvalRecord(item) {
      return {
        ledgerId: item.id,
        supplierCode: item.supplierCode,
        supplierName: item.supplierName,
        companyId: localstorageGet('companyId'),
        evalTime: this.formatCurrentDate(),
        evalLevel: '',
        // 首次合作起始日期：台账原始数据有值则带出并锁定禁用，无值由用户填写（可编辑）
        firstCooperationDate: item.firstCooperationDate || '',
        firstCooperationDateLocked: !!item.firstCooperationDate,
        recentPerformance: '',
        exceptionRecord: '',
        remark: '',
        // 年度评估专属附件（不区分类型）
        attachments: [],
        // —— 以下为表单展示字段（带入，禁止编辑）——
        cooperationType: item.cooperationType || '',
        storageDate: item.entryTime || '',
        businessScope: item.businessScope || ''
      };
    },

    // 当前日期字符串（YYYY-MM-DD），评估时间默认值
    formatCurrentDate() {
      const d = new Date();
      const pad = (n) => String(n).padStart(2, '0');
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
    },

    // 附件列表变化（EleUpload @change，含上传成功/删除）：仅保留已上传完成的文件，
    // 剔除 raw/status/percentage 等瞬态字段后按标准结构落库（fileName/fileUrl 等字段保证回显）
    handleAttachmentsChange(list) {
      if (!this.activeSupplier) return;
      this.activeSupplier.attachments = (list || [])
        .filter(item => item.status === 'uploaded')
        .map(item => ({
          fileName: item.name,
          fileUrl: item.fileUrl,
          absolutelyFileUrl: item.absolutelyFileUrl,
          fileId: item.fileId,
          id: item.id,
          fileType: item.fileType
        }));
    },

    // 左侧列表点击切换
    handleSelectSupplier(index) {
      this.activeIndex = index;
    },

    // 删除供应商评估记录：确认后从列表中移除，并修正选中项下标
    handleRemoveSupplier(index) {
      const supplierName = this.supplierList[index].supplierName || '-';
      this.$confirm(`确认要删除供应商【${supplierName}】的评估记录吗？`, '提示', { type: 'warning' })
        .then(() => {
          this.supplierList.splice(index, 1);
          // 修正选中下标：删的是当前选中则停留在原下标（超出取最后一条），删的是选中项之前的则前移一位
          if (this.activeIndex === index) {
            this.activeIndex = Math.min(index, this.supplierList.length - 1);
          } else if (index < this.activeIndex) {
            this.activeIndex -= 1;
          }
          // 列表删空时归零，右侧表单自动显示占位提示
          if (!this.supplierList.length) {
            this.activeIndex = 0;
          }
        })
        .catch(() => {});
    }
  }
};
</script>
<style lang='scss' scoped>
.supplier-access .supplier-form {
  ::v-deep {
    .el-textarea__inner {
      font-size: 14px !important;
    }
  }
}
.supplier-access {
  width: 100%;
  overflow: auto;
  background: #fff;
  padding: 16px 20px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 16px;

  // 页面主体：左侧列表 + 右侧表单
  .page-body {
    flex: 1;
    min-height: 0;
    display: flex;
    gap: 16px;

    // 左侧供应商评估列表
    .supplier-aside {
      width: 18%;
      min-width: 240px;
      display: flex;
      flex-direction: column;
      border: 1px solid #ebeef5;
      border-radius: 8px;

      .aside-title {
        flex-shrink: 0;
        padding: 2px 12px;
        font-size: 14px;
        font-weight: 600;
        color: #303133;
        border-bottom: 1px solid #ebeef5;
        background: #f5f7fa;
        border-radius: 8px 8px 0 0;

        // 标题下方的单次评估数量上限提示
        .aside-tip {
          margin-top: 2px;
          font-size: 12px;
          font-weight: 400;
          color: #909399;
          line-height: 16px;
        }
      }

      // 选择/添加供应商按钮区
      .aside-toolbar {
        flex-shrink: 0;
        padding: 8px 10px;
        border-bottom: 1px solid #ebeef5;

        .add-supplier-btn {
          width: 100%;
        }
      }

      .aside-list {
        flex: 1;
        min-height: 0;
        overflow: auto;

        .aside-item {
          display: flex;
          align-items: center;
          padding: 9px 12px;
          cursor: pointer;
          color: #606266;

          .aside-index {
            flex-shrink: 0;
            width: 16px;
            height: 16px;
            line-height: 16px;
            text-align: center;
            margin-right: 4px;
            border-radius: 50%;
            background: #f0f2f5;
            font-size: 12px;
          }

          .aside-name {
            flex: 1;
            // flex 子项默认 min-width:auto，长文本会撑开列表宽度；置 0 后超出部分省略显示
            min-width: 0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            font-size: 14px;
            text-align: left;
          }

          // 删除 icon：默认浅灰，hover 变红
          .remove-icon {
            flex-shrink: 0;
            margin-left: 8px;
            color: #c0c4cc;
            cursor: pointer;

            &:hover {
              color: #f56c6c;
            }
          }

          // 选中态：主题色
          &.active {
            color: #1989fa;
            background: #ecf5ff;

            .aside-index {
              background: #1989fa;
              color: #fff;
            }
          }

          &:hover {
            background: #f5f7fa;
          }

          &.active:hover {
            background: #ecf5ff;
          }
        }

        .aside-empty {
          padding: 32px 0;
          text-align: center;
          font-size: 14px;
          color: #909399;
        }
      }
    }

    // 右侧表单区
    .supplier-form {
      flex: 1;
      min-width: 0;
      overflow: auto;
      border: 1px solid #ebeef5;
      border-radius: 8px;
      padding: 12px;

      // 表单行容器：行内 form-item 等分宽度
      .form-row {
        display: flex;

        .form-item {
          flex: 1;
          min-width: 0;
          box-sizing: border-box;
          padding: 0 10px;

          // 日期/下拉控件占满格宽
          .form-control {
            width: 100%;
          }
        }
      }

      // —— 抵消 form-making 内置样式对组件内 el-form-item 的改写
      .form-row ::v-deep .el-form-item__label {
        width: 140px !important;
        padding: 0 12px 0 0 !important;
        text-align: right !important;
        font-size: 14px !important;
      }

      .form-row ::v-deep .el-form-item__content {
        width: auto !important;
        text-align: left !important;
        margin-bottom: 14px !important; // 与入库申请组件保持一致的表单项间距
      }

      .form-row ::v-deep .el-input__inner {
        text-align: left !important;
        font-size: 14px !important;
      }

      // 外层 form-making 字段校验失败时，.el-form-item.is-error 的后代选择器会把组件内
      // 所有输入框染红（含未报错的字段）；此处仅还原"自身未报错"的表单项边框
      .form-row ::v-deep .el-form-item:not(.is-error) {
        .el-input .el-input__inner {
          border-color: #DCDFE6;
        }

        .el-input.is-disabled .el-input__inner {
          border-color: #E4E7ED;
        }

        .el-textarea .el-textarea__inner {
          border-color: #DCDFE6;
        }
      }

      // 无数据占位
      .form-empty {
        height: 100%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 14px;
        color: #909399;
      }
    }
  }
}
// 选择供应商弹窗：已选数量提示（容器内常规布局） + 数据块两列布局
.selected-count {
  margin-bottom: 12px;
  font-size: 14px;
  color: #1989fa;
}

.supplier-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  max-height: 55vh;
  overflow: auto;

  .supplier-block {
    position: relative;
    border: 1px solid #ebeef5;
    border-radius: 6px;
    padding: 10px 12px;
    cursor: pointer;
    box-sizing: border-box;
    transition: border-color 0.2s, background-color 0.2s;

    &:hover {
      border-color: #1989fa;
    }

    // 选中态：主题色边框 + 浅蓝背景常规高亮
    &.selected {
      border-color: #1989fa;
      background: #ecf5ff;

      .block-check-icon {
        position: absolute;
        top: 8px;
        right: 8px;
        width: 18px;
        height: 18px;
        line-height: 18px;
        text-align: center;
        font-size: 12px;
        color: #fff;
        background: #1989fa;
        border-radius: 50%;
      }
    }

    .block-name {
      font-size: 14px;
      color: #303133;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      padding-right: 28px; // 给右上角对勾徽标留位
    }

    .block-code {
      margin-top: 4px;
      font-size: 12px;
      color: #909399;
    }
  }

  .supplier-empty {
    grid-column: 1 / -1;
    padding: 32px 0;
    text-align: center;
    font-size: 14px;
    color: #909399;
  }
}
</style>
