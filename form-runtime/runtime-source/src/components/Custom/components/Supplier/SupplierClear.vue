<!--
 * @Descripttion: 供应商清退（form-making 自定义控件：台账下拉选择 + 带入只读 + 清退信息填报）
 * @Author: junshao
-->
<template>
  <div class="supplier-clear" :style="rootStyle">
    <!-- 清退表单（单条记录，多列布局） -->
    <div class="supplier-form">
      <el-form v-if="formModel" :model="formModel" :rules="formRules" label-position="right" label-width="140px" size="small">
        <!-- 行1：供应商名称（台账下拉选择） + 归属公司（带入，禁用） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="供应商名称" prop="supplierName">
              <el-select v-model="formModel.supplierName" data-testid="a79ac339-0eaf-4963-9758-726166e84be8" :disabled="disabled" filterable placeholder="请选择供应商"
                class="form-control" @change="handleSupplierChange">
                <el-option v-for="item in ledgerOptions" :key="item.id" :label="item.supplierName"
                  :value="item.supplierName"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="归属公司" prop="companyId">
              <el-select v-model="formModel.companyId" data-testid="2f582a74-3456-410e-afd1-9a50738f56ca" disabled placeholder="请选择" class="form-control">
                <el-option v-for="opt in companyOptions" :key="opt.value" :label="opt.label" :value="opt.value"></el-option>
              </el-select>
            </el-form-item>
          </div>
        </div>
        <!-- 行2：供应商联系人 + 供应商联系电话（选择供应商后带入，不可编辑） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="供应商联系人" prop="contactPerson">
              <el-input v-model="formModel.contactPerson" data-testid="93cc4347-afe6-4cb2-8120-933ecb48b2aa" disabled placeholder="选择供应商后带入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="供应商联系电话" prop="contactPhone">
              <el-input v-model="formModel.contactPhone" data-testid="788fbabe-6ffd-4d5d-8936-4992153752cf" disabled placeholder="选择供应商后带入"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- 行3：统一社会信用代码 + 合作类型（选择供应商后带入，不可编辑） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="统一社会信用代码" prop="creditCode">
              <el-input v-model="formModel.creditCode" data-testid="c4482c5e-b6d0-4030-8ee4-24c98dd53ac4" disabled placeholder="选择供应商后带入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="合作类型" prop="cooperationType">
              <el-input v-model="formModel.cooperationType" data-testid="b6554e9c-cc9f-4626-b8d5-05f8cd803366" disabled placeholder="选择供应商后带入"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- 行4：合作起始日期（台账有值带入并禁用，无值可编辑） + 拟清退日期（不早于入库日期） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="合作起始日期" prop="cooperationStartDate">
              <el-date-picker v-model="formModel.cooperationStartDate" data-testid="c56c5900-c0d9-4ef9-814f-dea6c8af7494" :disabled="cooperationStartDisabled" type="date"
                value-format="yyyy-MM-dd" placeholder="请选择日期" class="form-control"></el-date-picker>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="拟清退日期" prop="plannedExitDate">
              <el-date-picker v-model="formModel.plannedExitDate" data-testid="21b17f73-7d3b-4d7e-9e0d-aec2d3b32f02" :disabled="disabled" type="date" value-format="yyyy-MM-dd"
                placeholder="请选择日期" :picker-options="plannedExitPickerOptions" class="form-control"></el-date-picker>
            </el-form-item>
          </div>
        </div>
        <!-- 行5：清退原因（整行） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="清退原因" prop="exitReason">
              <el-input v-model="formModel.exitReason" data-testid="331f894e-eda2-48ec-89fa-59490c76b5e4" :disabled="disabled" type="textarea" :autosize="{ minRows: 2 }" placeholder="请输入清退原因"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- 行6：附件上传（整行，EleUpload 组件：上传/预览/下载） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="附件上传" prop="attachments">
              <eleUpload :attachFile="formModel.attachments" data-testid="c4877be3-6f15-402c-a941-b28cffba60c8" :isDuplicateOverwrite="false" :size="300" :uploadLimit="20" :showOnly="disabled" @change="handleAttachmentsChange"></eleUpload>
            </el-form-item>
          </div>
        </div>
        <!-- 行7：备注（整行） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="备注" prop="remark">
              <el-input v-model="formModel.remark" data-testid="3e46c067-a1ca-4a88-9398-762c92be7183" :disabled="disabled" type="textarea" :autosize="{ minRows: 2 }" placeholder="请输入备注信息"></el-input>
            </el-form-item>
          </div>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script>
import Api from '@/api';
import eleUpload from '@/components/EleUpload';
import { localstorageGet } from '@/utils/auth';

// 空表单模型（含 payload 字段 + 带入展示字段）
function createEmptyModel() {
  return {
    // ---- 提交 payload 字段 ----
    ledgerId: '', // 入库台账ID
    supplierCode: '', // 供应商编码
    supplierName: '', // 供应商名称
    companyId: '', // 所属公司ID
    exitReason: '', // 清退原因
    plannedExitDate: '', // 拟清退日期
    cooperationStartDate: '', // 合作起始日期
    remark: '', // 备注
    attachments: [], // 附件
    // ---- 带入展示字段（随 payload 一并存储，用于审批/回显还原展示） ----
    contactPerson: '', // 供应商联系人
    contactPhone: '', // 供应商联系电话
    creditCode: '', // 统一社会信用代码
    cooperationType: '' // 合作类型
  };
}

// 解析 form-making 传入的 value（JSON 字符串 -> 单条表单对象），空值/异常兜底为空模型
function safeParseModel(val) {
  if (!val) return createEmptyModel();
  try {
    const parsed = JSON.parse(val);
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return Object.assign(createEmptyModel(), parsed);
    }
    return createEmptyModel();
  } catch (error) {
    return createEmptyModel();
  }
}

export default {
  name: 'SupplierClear',
  components: { eleUpload },
  props: {
    // form-making v-model 通道：JSON.stringify(单条表单对象)，未选择供应商为空字符串
    value: {
      type: String,
      default: ''
    },
    // 审批态：控件全部禁用
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
      formModel: safeParseModel(this.value),
      // 供应商入库台账下拉选项
      ledgerOptions: [],
      // 审批/打印态归属公司反显用：全量公司列表（发起态不加载）
      allCompanyList: [],
      // 内部逻辑字段（不进 payload）：入库日期（拟清退日期下限）、台账合作起始日期原值（有值则带入禁用）；
      // 始终由台账记录实时推导，草稿回显时在台账加载完成后恢复
      ledgerContext: {
        storageDate: '',
        firstCooperationDate: ''
      },
      // 除附件与备注外全部必填
      formRules: {
        supplierName: [{ required: true, message: '请选择供应商名称', trigger: 'change' }],
        companyId: [{ required: true, message: '请选择归属公司', trigger: 'change' }],
        contactPerson: [{ required: true, message: '请输入供应商联系人', trigger: 'blur' }],
        contactPhone: [{ required: true, message: '请输入供应商联系电话', trigger: 'blur' }],
        creditCode: [{ required: true, message: '请输入统一社会信用代码', trigger: 'blur' }],
        cooperationType: [{ required: true, message: '请输入合作类型', trigger: 'blur' }],
        cooperationStartDate: [{ required: true, message: '请选择合作起始日期', trigger: 'change' }],
        plannedExitDate: [{ required: true, message: '请选择拟清退日期', trigger: 'change' }],
        exitReason: [{ required: true, message: '请输入清退原因', trigger: 'blur' }]
      }
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
    // 合作起始日期：台账带入值后禁用（台账无值时由发起人补填）
    cooperationStartDisabled() {
      return this.disabled || !!this.ledgerContext.firstCooperationDate;
    },
    // 拟清退日期：不能早于入库日期（未选供应商时暂不限制）
    plannedExitPickerOptions() {
      const storageDate = this.ledgerContext.storageDate;
      if (!storageDate) return {};
      return {
        disabledDate(time) {
          return time.getTime() < new Date(storageDate).getTime();
        }
      };
    }
  },
  watch: {
    // 外部值变化（如 setData 回显、流程草稿恢复）时重新装载；
    // 与当前数据一致时跳过，避免组件自身 emit 引发的回环覆盖
    value(val) {
      const parsed = safeParseModel(val);
      if (JSON.stringify(parsed) === JSON.stringify(this.formModel)) return;
      this.formModel = parsed;
      // 草稿回显/外部设值后，按台账记录恢复内部联动字段
      this.syncLedgerContext();
    },
    // 数据变化同步回 form-making；未选择供应商时发空字符串，
    // 保证设计器配置的"必填"校验能真正拦截空数据
    formModel: {
      handler(model) {
        const payload = model.ledgerId || model.supplierName ? JSON.stringify(model) : '';
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
    // 查询当前用户所属公司的供应商台账（与评估组件同一接口）
    loadLedgerList() {
      // 审批态不可编辑，无需拉取
      if (this.disabled) return;
      this.$axios.post(
        Api.supplierCenter.getSupplierList,
        { data: { companyId: localstorageGet('companyId'), ledgerStatus: 1, current: 1, size: 9999 } },
        (res) => {
          if (!res.isSuccess) return;
          this.ledgerOptions = (res.data && res.data.records) || [];
          // 台账加载完成后，按当前已选供应商恢复内部联动字段（草稿回显场景）
          this.syncLedgerContext();
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

    // 选择供应商：按台账记录带入相关字段；
    // 发起人填写内容（清退原因/拟清退日期/附件/备注）切换供应商时一并重置
    handleSupplierChange(supplierName) {
      const ledger = this.ledgerOptions.find(item => item.supplierName === supplierName);
      if (!ledger) return;
      Object.assign(this.formModel, {
        ledgerId: ledger.id,
        supplierCode: ledger.supplierCode || '',
        supplierName: ledger.supplierName || '',
        companyId: localstorageGet('companyId'),
        contactPerson: ledger.contactPerson || '',
        contactPhone: ledger.contactPhone || '',
        creditCode: ledger.unifiedCreditCode || '',
        cooperationType: ledger.cooperationType || '',
        cooperationStartDate: ledger.firstCooperationDate || '',
        // —— 发起人填写内容重置 ——
        plannedExitDate: '',
        exitReason: '',
        attachments: [],
        remark: ''
      });
      // 同步内部联动字段（入库日期下限/台账合作起始日期原值）
      this.syncLedgerContext();
    },

    // 依据当前已选台账记录同步内部联动字段（入库日期/台账合作起始日期原值）；
    // 这两个字段只参与联动逻辑、不进 payload，切换供应商、草稿回显、台账加载完成后均需重新推导
    syncLedgerContext() {
      const ledger = this.ledgerOptions.find(item =>
        (this.formModel.ledgerId && item.id === this.formModel.ledgerId) ||
        (!this.formModel.ledgerId && this.formModel.supplierName && item.supplierName === this.formModel.supplierName)
      );
      this.ledgerContext = ledger
        ? {
          // 入库时间为 "YYYY-MM-DD HH:mm:ss"，取日期部分作为拟清退日期下限
          storageDate: (ledger.entryTime || '').split(' ')[0] || '',
          firstCooperationDate: ledger.firstCooperationDate || ''
        }
        : { storageDate: '', firstCooperationDate: '' };
    },

    // 附件列表变化（EleUpload @change，含上传成功/删除）：仅保留已上传完成的文件，
    // 剔除 raw/status/percentage 等瞬态字段后按 EleUpload 标准结构落库（fileName/fileUrl 等字段保证回显）
    handleAttachmentsChange(list) {
      this.formModel.attachments = (list || [])
        .filter(item => item.status === 'uploaded')
        .map(item => ({
          fileName: item.name,
          fileUrl: item.fileUrl,
          absolutelyFileUrl: item.absolutelyFileUrl,
          fileId: item.fileId,
          id: item.id,
          fileType: item.fileType
        }));
    }
  }
};
</script>
<style lang='scss' scoped>
.supplier-clear .supplier-form {
  ::v-deep {
    .el-textarea__inner {
      font-size: 14px !important;
    }
  }
}
.supplier-clear {
  width: 100%;
  overflow: auto;
  background: #fff;
  padding: 16px 20px;
  box-sizing: border-box;

  // 清退表单区
  .supplier-form {
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
      width: 140px !important; // 与 el-form 的 label-width="140px" 保持一致
      padding: 0 12px 0 0 !important;
      text-align: right !important;
      font-size: 14px !important;
    }

    .form-row ::v-deep .el-form-item__content {
      width: auto !important;
      text-align: left !important;
      margin-bottom: 14px !important; // 与评估组件保持一致的表单项间距
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
  }
}
</style>
