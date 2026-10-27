<!--
 * @Descripttion: 黑名单供应商重新入库（form-making 自定义控件：已清退黑名单下拉选择 + 带入只读 + 重新入库说明填报）
 * @Author: junshao
-->
<template>
  <div class="supplier-re-entry">
    <!-- 重新入库表单（单条记录，多列布局） -->
    <div class="supplier-form">
      <el-form v-if="formModel" :model="formModel" :rules="formRules" label-position="right" label-width="150px" size="small">
        <!-- 行1：供应商名称（已清退黑名单下拉选择） + 归属公司（带入，禁用） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="供应商名称" prop="supplierName">
              <el-select v-model="formModel.supplierName" data-testid="5d1a9f9b-c63c-4d05-8c8c-bec14adae0c0" :disabled="disabled" filterable placeholder="请选择供应商"
                class="form-control" @change="handleSupplierChange">
                <el-option v-for="item in blacklistOptions" :key="item.id" :label="item.supplierName"
                  :value="item.supplierName"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="归属公司" prop="companyId">
              <el-select v-model="formModel.companyId" data-testid="3999b603-c3c3-4fd8-b9cf-e60abf749d9b" disabled placeholder="请选择" class="form-control">
                <el-option v-for="opt in companyOptions" :key="opt.value" :label="opt.label" :value="opt.value"></el-option>
              </el-select>
            </el-form-item>
          </div>
        </div>
        <!-- 行2：成立日期 + 注册资本（选择供应商后带入，禁用） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="成立日期" prop="establishDate">
              <el-input v-model="formModel.establishDate" data-testid="8b099e5f-e4ed-498e-b69d-31a61354350b" disabled placeholder="成立日期"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="注册资本" prop="registeredCapital">
              <el-input v-model="formModel.registeredCapital" data-testid="c32077e4-3cc0-442c-82cf-7c5e6359556d" disabled placeholder="注册资本"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- 行3：供应商联系人 + 供应商联系方式（选择供应商后带入，禁用） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="供应商联系人" prop="contactPerson">
              <el-input v-model="formModel.contactPerson" data-testid="4481e35c-4c30-4b06-b612-f54967eb1904" disabled placeholder="供应商联系人，选择供应商后带入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="供应商联系方式" prop="contactPhone">
              <el-input v-model="formModel.contactPhone" data-testid="65d8c6a1-51a9-4014-b249-9e2667d72ad9" disabled placeholder="供应商联系方式，选择供应商后带入"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- 行4：原清退时间 + 合作类型（选择供应商后带入，禁用） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="原清退时间" prop="originalExitTime">
              <el-input v-model="formModel.originalExitTime" data-testid="22e2e4b9-1c4b-470a-b26e-c67ea106b5a2" disabled placeholder="原清退时间，选择供应商后带入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="合作类型" prop="cooperationType">
              <el-input v-model="formModel.cooperationType" data-testid="6befb44f-98d0-4821-bee9-6073bf09f898" disabled placeholder="合作类型，选择供应商后带入"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- 行5：经营范围及资质情况（整行，带入禁用） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="经营范围及资质情况" prop="businessScopeAndQualification">
            <el-input v-model="formModel.businessScopeAndQualification" data-testid="baad3117-847f-4ae5-8f06-c5acd1752362" disabled type="textarea" :autosize="{ minRows: 2 }"
              placeholder="经营范围及资质情况，选择供应商后带入"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- 行6：原清退原因（整行，带入禁用） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="原清退原因" prop="originalExitReason">
            <el-input v-model="formModel.originalExitReason" data-testid="8960b685-23e7-4616-93cf-45f981c8749a" disabled type="textarea" :autosize="{ minRows: 2 }"
              placeholder="原清退原因，选择供应商后带入"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- 行7：申请重新入库说明（整行，发起人填写） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="申请重新入库说明" prop="reentryReason">
            <el-input v-model="formModel.reentryReason" data-testid="2a30a14b-500b-495c-b84b-e3364106cb74" :disabled="disabled" type="textarea" :autosize="{ minRows: 2 }"
              placeholder="请输入申请重新入库说明"></el-input>
            </el-form-item>
          </div>
        </div>
        <!-- 行8：备注（整行，发起人填写） -->
        <div class="form-row">
          <div class="form-item">
            <el-form-item label="备注" prop="remark">
            <el-input v-model="formModel.remark" data-testid="acfccf07-4cc0-4699-a3dc-bac98e58fd9a" :disabled="disabled" type="textarea" :autosize="{ minRows: 2 }"
              placeholder="请输入备注信息"></el-input>
            </el-form-item>
          </div>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script>
import Api from '@/api';
import { localstorageGet } from '@/utils/auth';

// 空表单模型（含 payload 字段 + 带入展示字段）
function createEmptyModel() {
  return {
    // ---- 提交 payload 字段 ----
    ledgerId: '', // 入库台账ID
    supplierCode: '', // 供应商编码
    supplierName: '', // 供应商名称
    companyId: '', // 所属公司ID
    originalExitTime: '', // 原清退时间（台账 exitTime）
    originalExitReason: '', // 原清退原因（台账 exitReason）
    reentryReason: '', // 申请重新入库说明
    cooperationType: '', // 合作类型
    remark: '', // 备注
    // ---- 带入展示字段（随 payload 一并存储，用于审批/回显还原展示） ----
    establishDate: '', // 成立日期
    registeredCapital: '', // 注册资本
    contactPerson: '', // 供应商联系人
    contactPhone: '', // 供应商联系方式
    businessScopeAndQualification: '' // 经营范围及资质情况（台账经营范围 + 资质情况合并带入）
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
  name: 'SupplierReEntry',
  components: {},
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
      // 已清退的黑名单供应商下拉选项
      blacklistOptions: [],
      // 审批/打印态归属公司反显用：全量公司列表（发起态不加载）
      allCompanyList: [],
      // 除备注外全部必填
      formRules: {
        supplierName: [{ required: true, message: '请选择供应商名称', trigger: 'change' }],
        companyId: [{ required: true, message: '请选择归属公司', trigger: 'change' }],
        establishDate: [{ required: true, message: '请输入成立日期', trigger: 'blur' }],
        registeredCapital: [{ required: true, message: '请输入注册资本', trigger: 'blur' }],
        contactPerson: [{ required: true, message: '请输入供应商联系人', trigger: 'blur' }],
        contactPhone: [{ required: true, message: '请输入供应商联系方式', trigger: 'blur' }],
        originalExitTime: [{ required: true, message: '请输入原清退时间', trigger: 'blur' }],
        cooperationType: [{ required: true, message: '请输入合作类型', trigger: 'blur' }],
        businessScopeAndQualification: [{ required: true, message: '请输入经营范围及资质情况', trigger: 'blur' }],
        originalExitReason: [{ required: true, message: '请输入原清退原因', trigger: 'blur' }],
        reentryReason: [{ required: true, message: '请输入申请重新入库说明', trigger: 'blur' }]
      }
    };
  },
  computed: {
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
    }
  },
  watch: {
    // 外部值变化（如 setData 回显、流程草稿恢复）时重新装载；
    // 与当前数据一致时跳过，避免组件自身 emit 引发的回环覆盖
    value(val) {
      const parsed = safeParseModel(val);
      if (JSON.stringify(parsed) === JSON.stringify(this.formModel)) return;
      this.formModel = parsed;
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
    this.loadBlackList();
    // 审批/打印态归属公司反显数据源
    this.loadCompanyListForView();
  },
  mounted() { },
  methods: {
    // 查询已清退的黑名单供应商（无额外参数）
    loadBlackList() {
      // 审批态不可编辑，无需拉取
      if (this.disabled) return;
      this.$axios.post(
        Api.supplierCenter.getBlackListExitedSupplierList,
        {
          data: {
            companyId: localstorageGet('companyId')
          }
        },
        (res) => {
          if (!res.isSuccess) {
            this.$message.error(res.message || '查询黑名单供应商失败');
            return;
          };
          // 兼容直接返回数组或分页结构两种响应形态
          const list = Array.isArray(res.data?.data) ? res.data.data : [];
          this.blacklistOptions = list;
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

    // 选择供应商：按黑名单记录带入相关字段（字段命名与供应商台账一致）；
    // 发起人填写内容（申请重新入库说明/备注）切换供应商时一并重置
    handleSupplierChange(supplierName) {
      const record = this.blacklistOptions.find(item => item.supplierName === supplierName);
      if (!record) return;
      Object.assign(this.formModel, {
        ledgerId: record.id,
        supplierCode: record.supplierCode || '',
        supplierName: record.supplierName || '',
        companyId: localstorageGet('companyId'),
        originalExitTime: record.exitTime || '',
        originalExitReason: record.exitReason || '',
        cooperationType: record.cooperationType || '',
        establishDate: record.establishmentDate || '',
        registeredCapital: record.registeredCapital || '',
        contactPerson: record.contactPerson || '',
        contactPhone: record.contactPhone || '',
        businessScopeAndQualification: record.businessScopeAndQualification || '',
        // —— 发起人填写内容重置 ——
        reentryReason: '',
        remark: ''
      });
    }
  }
};
</script>
<style lang='scss' scoped>
.supplier-re-entry .supplier-form {
  ::v-deep {
    .el-textarea__inner {
      font-size: 14px !important;
    }
  }
}
.supplier-re-entry {
  width: 100%;
  overflow: auto;
  background: #fff;
  padding: 16px 20px;
  box-sizing: border-box;

  // 重新入库表单区
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
      width: 150px !important; // 与 el-form 的 label-width="150px" 保持一致
      padding: 0 12px 0 0 !important;
      text-align: right !important;
      font-size: 14px !important;
    }

    .form-row ::v-deep .el-form-item__content {
      width: auto !important;
      text-align: left !important;
      margin-bottom: 14px !important; // 与评估/清退组件保持一致的表单项间距
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
