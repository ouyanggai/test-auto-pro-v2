<!--
 * @Descripttion: 供应商变更（form-making 自定义控件：台账下拉选择供应商带入数据 + 信息变更填报，字段与入库申请一致）
 * @Author: junshao
-->
<template>
  <div class="supplier-change">
    <!-- 变更表单（单条记录，两列网格布局，与入库申请右侧表单一致） -->
    <div class="supplier-form">
      <el-form ref="supplierForm" :model="formModel" label-position="right" label-width="160px" size="small">
        <div class="form-grid">
          <div class="form-item">
            <el-form-item label="供应商名称" prop="supplierName" :rules="[{ required: true, message: '请选择供应商名称', trigger: 'change' }]">
              <div style="display: flex;">
                <el-select v-model="formModel.supplierName" data-testid="88c223e2-e21d-4dba-b52f-43a3d8e06310" :disabled="isSupplierNameDisabled" filterable placeholder="请选择供应商"
                  style="flex: 1;" @change="handleSupplierChange">
                  <el-option v-for="item in ledgerOptions" :key="item.id" :label="item.changeStatus === '变更中' ? item.supplierName + '（变更中）' : item.supplierName" :value="item.supplierName" :disabled="item.changeStatus === '变更中'"></el-option>
                </el-select>
                <el-tag v-if="!disabled" :type="verifyStatusTag.type" effect="dark" size="small"
                  style="margin-left: 10px; flex-shrink: 0; align-self: center;">{{ verifyStatusTag.text }}</el-tag>
              </div>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="归属公司" prop="companyId" :rules="[{ required: true, message: '请选择归属公司', trigger: 'change' }]">
              <el-select v-model="formModel.companyId" data-testid="7fae0e60-51fb-4180-98b1-8d28887fe5cd" disabled placeholder="请选择" class="form-control">
                <el-option v-for="opt in companyOptions" :key="opt.value" :label="opt.label" :value="opt.value"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="统一社会信用代码" prop="unifiedCreditCode" :rules="creditCodeRules">
              <div style="display: flex;">
                <el-input v-model="formModel.unifiedCreditCode" data-testid="01d510b6-318b-455f-a1f6-d481353b634d" disabled placeholder="选择供应商后带入" style="flex: 1;"></el-input>
                <el-button v-if="!disabled" type="primary" data-testid="15fafc6b-5b5f-429f-872d-a1ecefd80d01" @click="handleVerifySupplier" style="margin-left: 10px;width: 100px;">工商数据校验</el-button>
              </div>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="公司地址" prop="companyAddress" :rules="[{ required: true, message: '请输入公司地址', trigger: 'blur' }]">
              <el-input v-model="formModel.companyAddress" data-testid="9a27bd7e-111f-44f0-868e-ff2e83897d68" :disabled="isBusinessFieldDisabled('companyAddress')" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="级别" prop="supplierLevel" :rules="[{ required: true, message: '请选择级别', trigger: 'change' }]">
              <el-select v-model="formModel.supplierLevel" data-testid="32cf2133-0cf9-4819-b349-4e731e4d3348" :disabled="disabled" placeholder="请选择" class="form-control">
                <el-option v-for="opt in levelOptions" :key="opt" :label="opt" :value="opt"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="合作类型" prop="cooperationType" :rules="[{ required: true, message: '请选择合作类型', trigger: 'change' }]">
              <el-select v-model="formModel.cooperationType" data-testid="173a4e6f-e840-4ef1-a89e-7366220cb588" :disabled="disabled" placeholder="请选择" class="form-control">
                <el-option v-for="opt in cooperationTypeOptions" :key="opt" :label="opt" :value="opt"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="一级大类" prop="categoryId" :rules="[{ required: true, message: '请选择一级大类', trigger: 'change' }]">
              <el-select v-model="formModel.categoryId" data-testid="79847b4f-acb3-431d-a39a-1454cfd84d5b" :disabled="disabled" placeholder="请选择" class="form-control" @change="handleFirstCategoryChange">
                <el-option v-for="opt in firstCategoryList" :key="opt.id" :label="opt.name" :value="opt.id"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="二级品类" prop="subCategoryId" :rules="[{ required: true, message: '请选择二级品类', trigger: 'change' }]">
              <el-select v-model="formModel.subCategoryId" data-testid="7d170650-d60a-4b18-a44b-a442d8f181b2" :disabled="disabled" placeholder="请选择" class="form-control">
                <el-option v-for="opt in secondCategoryOptions" :key="opt.id" :label="opt.name" :value="opt.id"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="注册资本" prop="registeredCapital" :rules="[{ required: true, message: '请输入注册资本', trigger: 'blur' }]">
              <el-input v-model="formModel.registeredCapital" data-testid="0a9d735a-6c44-4d8e-a4f1-6ae6ecab4586" :disabled="isBusinessFieldDisabled('registeredCapital')" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="成立日期" prop="establishmentDate" :rules="[{ required: true, message: '请选择成立日期', trigger: 'change' }]">
              <el-date-picker v-model="formModel.establishmentDate" data-testid="7f53e647-d8a3-4e17-9316-952f5ee7c41a" :disabled="isBusinessFieldDisabled('establishmentDate')" type="date" value-format="yyyy-MM-dd" placeholder="请选择日期"
                class="form-control"></el-date-picker>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="法定代表人" prop="legalRepresentative" :rules="[{ required: true, message: '请输入法定代表人', trigger: 'blur' }]">
              <el-input v-model="formModel.legalRepresentative" data-testid="b014fbc6-3ab8-4935-bdd2-f3a646ca02ad" :disabled="isBusinessFieldDisabled('legalRepresentative')" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="资质等级" prop="qualificationLevel">
              <el-select v-model="formModel.qualificationLevel" data-testid="681e3f81-7ff5-4a3c-95a9-a4abb262f0a8" :disabled="disabled" placeholder="请选择" class="form-control">
                <el-option v-for="opt in qualificationLevelOptions" :key="opt" :label="opt" :value="opt"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="主体资质名称" prop="mainQualificationName">
              <el-input v-model="formModel.mainQualificationName" data-testid="557986bb-3d8b-490a-8d2a-c83157482c32" :disabled="disabled" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="主体资质有效期" prop="mainQualificationValidity">
              <el-date-picker v-model="formModel.mainQualificationValidity" data-testid="ade817f2-44b4-42ad-8764-61941e84a0a6" :disabled="disabled" type="date" value-format="yyyy-MM-dd" :picker-options="validityPickerOptions" placeholder="请选择日期"
                class="form-control"></el-date-picker>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="安全生产许可证编号" prop="safetyLicenseNo">
              <el-input v-model="formModel.safetyLicenseNo" data-testid="37f9ab4d-09e9-4f48-bce4-2a52840aacec" :disabled="disabled" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="安全生产许可证有效期" prop="safetyLicenseValidity">
              <el-date-picker v-model="formModel.safetyLicenseValidity" data-testid="bdabc9ee-e564-4bd0-9419-c152879bedbb" :disabled="disabled" type="date" value-format="yyyy-MM-dd" :picker-options="validityPickerOptions" placeholder="请选择日期"
                class="form-control"></el-date-picker>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="人员资质情况" prop="personnelQualification">
              <el-input v-model="formModel.personnelQualification" data-testid="f81f21c8-be20-46d1-8857-ec8e862f7c05" :disabled="disabled" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="三标体系认证" prop="threeSystemCert">
              <el-select v-model="formModel.threeSystemCert" data-testid="40b8a09f-e236-4778-bc82-6fc972e7e96d" :disabled="disabled" placeholder="请选择" class="form-control">
                <el-option label="是" value="是"></el-option>
                <el-option label="否" value="否"></el-option>
              </el-select>
            </el-form-item>
          </div>
          <div class="form-item full-row">
            <el-form-item label="信用情况" prop="creditStatus" :rules="[{ required: true, message: '请输入信用情况', trigger: 'blur' }]">
              <el-input v-model="formModel.creditStatus" data-testid="740c8441-de0d-4f4e-a4ca-2ddb248921b9" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item full-row">
            <el-form-item label="业绩情况" prop="performanceStatus" :rules="[{ required: true, message: '请输入业绩情况', trigger: 'blur' }]">
              <el-input v-model="formModel.performanceStatus" data-testid="0d5b3399-f748-42b1-9fa6-1844606badfb" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="供应商联系人" prop="contactPerson" :rules="[{ required: true, message: '请输入供应商联系人', trigger: 'blur' }]">
              <el-input v-model="formModel.contactPerson" data-testid="433b3e36-ffc5-433e-9640-364be349f01d" :disabled="disabled" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <div class="form-item">
            <el-form-item label="供应商联系方式" prop="contactPhone" :rules="contactPhoneRules">
              <el-input v-model="formModel.contactPhone" data-testid="a1a9cccc-49e8-46b1-908e-b8463ea241c8" :disabled="disabled" placeholder="请输入" maxlength="11"></el-input>
            </el-form-item>
          </div>
          <div class="form-item full-row">
            <el-form-item label="经营范围" prop="businessScope" :rules="[{ required: true, message: '请输入经营范围', trigger: 'blur' }]">
              <el-input v-model="formModel.businessScope" data-testid="d7b27810-1dca-4e21-b209-0f76782506d2" :disabled="isBusinessFieldDisabled('businessScope')" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
          <!-- 附件全空提示：仅在可编辑态展示 -->
          <!-- <div v-if="!disabled && attachmentsEmpty" class="form-item full-row">
            <div class="attachment-required-tip"><i class="el-icon-info"></i> 至少选择一种类型的附件进行上传</div>
          </div> -->
          <!-- 附件按类型区分：三种类型各带上传按钮（均必填），文件统一存 attachments（带 attachmentType 属性）；回显态隐藏按钮仅展示 -->
          <div class="form-item full-row" v-for="t in attachmentFields" :key="t.value">
            <el-form-item :label="t.label" prop="attachments" :rules="typedAttachmentRules(t)">
                <eleUpload :attachFile="attachmentsByType[t.value]" :data-testid="attachmentTestIdMap[t.value]" :isDuplicateOverwrite="false" :size="300" :uploadLimit="20" :showOnly="disabled" :isRealDelete="false" @change="list => handleTypedAttachmentsChange(t.value, list)"></eleUpload>
              </el-form-item>
          </div>
          <div class="form-item full-row">
            <el-form-item label="备注" prop="remark">
              <el-input v-model="formModel.remark" data-testid="5ff386e3-564b-4937-85c2-7a92807cae5f" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入"></el-input>
            </el-form-item>
          </div>
        </div>
        <div class="change-panel" v-if="displayChangedFields.length">
          <div class="change-panel-title">{{`本次变更内容（共 ${displayChangedFields.length} 项）`}}</div>
          <div class="change-table-container">
            <el-table :data="displayChangedFields" size="mini">
              <el-table-column prop="label" label="字段名" width="150"></el-table-column>
              <el-table-column prop="oldValue" label="原值" min-width="180" show-overflow-tooltip></el-table-column>
              <el-table-column prop="newValue" label="修改值" min-width="180" show-overflow-tooltip></el-table-column>
            </el-table>
          </div>
        </div>
        <!-- 变更原因（必填） -->
        <div class="change-reason-row">
          <el-form-item label="变更原因" prop="changeReason" :rules="[{ required: true, message: '请输入变更原因', trigger: 'blur' }]">
            <el-input v-model="formModel.changeReason" data-testid="845068ab-c74a-414a-9e8b-c2e2d3311b0b" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入变更原因"></el-input>
          </el-form-item>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script>
import Api from '@/api';
import eleUpload from '@/components/EleUpload';
import { localstorageGet } from '@/utils/auth';

// 空表单模型（字段与入库申请表单一一对应，另含台账定位字段与校验状态/差异记录等辅助字段）
function createEmptyModel() {
  return {
    ledgerId: '', // 入库台账ID（选中供应商带入，审批通过后据此定位台账记录）
    supplierCode: '', // 供应商编码（选中供应商带入）
    supplierName: '', // 供应商名称
    companyId: '', // 归属公司ID
    unifiedCreditCode: '', // 统一社会信用代码
    companyAddress: '', // 公司地址
    supplierLevel: '', // 级别
    cooperationType: '', // 合作类型
    categoryId: '', // 一级大类ID
    subCategoryId: '', // 二级品类ID
    registeredCapital: '', // 注册资本
    establishmentDate: '', // 成立日期
    legalRepresentative: '', // 法定代表人
    qualificationLevel: '', // 资质等级
    mainQualificationName: '', // 主体资质名称
    mainQualificationValidity: '', // 主体资质有效期
    safetyLicenseNo: '', // 安全生产许可证编号
    safetyLicenseValidity: '', // 安全生产许可证有效期
    personnelQualification: '', // 人员资质情况
    threeSystemCert: '', // 三标体系认证
    creditStatus: '', // 信用情况
    performanceStatus: '', // 业绩情况
    contactPerson: '', // 供应商联系人
    contactPhone: '', // 供应商联系方式
    businessScope: '', // 经营范围
    attachments: [], // 附件（按 attachmentType 区分类型的文件对象数组）
    remark: '', // 备注
    changeReason: '', // 变更原因（必填）
    verifyStatus: 'unverified', // 工商校验状态（success 后锁定工商信息字段）
    diffFields: [] // 工商校验自动更新的差异字段
  };
}

// 解析 form-making 传入的 value（JSON 字符串 -> 单条表单对象），空值/异常兜底为空模型
function safeParseModel(val) {
  if (!val) return createEmptyModel();
  try {
    const parsed = JSON.parse(val);
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      // changeDetails / changedFields 为派生数据，不回灌表单模型，避免与计算值回环
      delete parsed.changeDetails;
      delete parsed.changedFields;
      return Object.assign(createEmptyModel(), parsed);
    }
    return createEmptyModel();
  } catch (error) {
    return createEmptyModel();
  }
}

// 提取已保存的变更内容列表（展示格式），审批/回显态渲染变更内容面板用
function extractSavedChangedFields(val) {
  if (!val) return [];
  try {
    const parsed = JSON.parse(val);
    return parsed && Array.isArray(parsed.changedFields) ? parsed.changedFields : [];
  } catch (error) {
    return [];
  }
}

// 驼峰字段名 -> 下划线字段名（后端数据库字段为下划线命名，如 contactPerson -> contact_person）
function camelToSnake(key) {
  return key.replace(/([A-Z])/g, '_$1').toLowerCase();
}

export default {
  name: 'SupplierChange',
  components: { eleUpload },
  inject: {
    // $initFlow 透传的 options：台账"申请变更"入口携台账 id 默认选中；常规流程入口无注入取 null
    initFlowDialogOptions: { default: null }
  },
  props: {
    // form-making v-model 通道：JSON.stringify(单条表单对象)，未填写关键信息为空字符串
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
    }
  },
  data() {
    return {
      formModel: safeParseModel(this.value),
      // 已保存的变更内容列表（展示格式，从表单数据中还原）：审批/回显态渲染变更内容面板用
      savedChangedFields: extractSavedChangedFields(this.value),
      // 供应商入库台账下拉选项（供应商名称选择用）
      ledgerOptions: [],
      // 审批/打印态归属公司反显用：全量公司列表（发起态不加载）
      allCompanyList: [],
      // 台账"申请变更"入口经 $initFlow 注入的台账 id，下拉选项就绪后默认选中
      defaultLedgerId: '',
      // 默认选中是否已尝试过（台账/大类两个异步源都会触发，只执行一次）
      defaultLedgerApplied: false,
      // 参与变更对比的供应商字段清单（key -> 中文名，顺序即变更对比列表的展示顺序）
      compareFieldMap: {
        supplierName: '供应商名称',
        unifiedCreditCode: '统一社会信用代码',
        companyAddress: '公司地址',
        supplierLevel: '级别',
        cooperationType: '合作类型',
        categoryId: '一级大类',
        subCategoryId: '二级品类',
        registeredCapital: '注册资本',
        establishmentDate: '成立日期',
        legalRepresentative: '法定代表人',
        qualificationLevel: '资质等级',
        mainQualificationName: '主体资质名称',
        mainQualificationValidity: '主体资质有效期',
        safetyLicenseNo: '安全生产许可证编号',
        safetyLicenseValidity: '安全生产许可证有效期',
        personnelQualification: '人员资质情况',
        threeSystemCert: '三标体系认证',
        creditStatus: '信用情况',
        performanceStatus: '业绩情况',
        contactPerson: '供应商联系人',
        contactPhone: '供应商联系方式',
        businessScope: '经营范围',
        attachments: '附件',
        remark: '备注'
      },
      // 原始供应商数据快照（选中/带入供应商时保存，与表单数据隔离，仅用于变更对比）
      originalData: null,
      levelOptions: ['一级', '二级', '三级'],
      // 合作类型下拉选项
      cooperationTypeOptions: ['供应商', '施工商', '服务商', '其他类'],
      firstCategoryList: [],
      secondCategoryOptions: [],
      // 资质等级下拉选项
      qualificationLevelOptions: ['甲级', '乙级', '一级', '二级', '三级'],
      // 有效期字段（主体资质/安全生产许可证）日期面板快捷选项：「长期」固定写入 9999-12-31
      validityPickerOptions: {
        shortcuts: [
          {
            text: '长期',
            onClick(picker) {
              picker.$emit('pick', new Date(9999, 11, 31));
            }
          }
        ]
      },
      // 附件分区 data-testid 映射（每个分区独立id，自动化测试按分区精确定位；新增分区时在此补充对应id）
      attachmentTestIdMap: {
        1: 'd4750fc0-871d-42de-83c5-e89454e2b66b',
        2: 'da95acc6-24ad-48af-90e5-450761b61833',
        3: '9c8cb42b-b07d-4d66-8a03-a1d1f382d360'
      },
      // 附件类型字段（按类型区分上传；文件统一存 attachments，带 attachmentType 属性；required 控制是否必填）
      attachmentFields: [
        { label: '企业介绍附件', value: 1, required: true },
        { label: '经营业绩附件', value: 2, required: true },
        { label: '考察报告附件', value: 3, required: true }
      ],
      // 工商校验返回的 businessData 字段 -> 表单字段（比对与差异回填用）
      businessFieldMap: {
        companyName: 'supplierName',
        creditCode: 'unifiedCreditCode',
        legalRepresentative: 'legalRepresentative',
        registeredCapital: 'registeredCapital',
        establishmentDate: 'establishmentDate',
        companyAddress: 'companyAddress',
        businessScope: 'businessScope'
      },
      // 工商信息字段清单（与入库申请 previewColumns 一致；校验成功后这些字段锁定编辑）
      verifiedLockKeys: [
        'supplierName',
        'unifiedCreditCode',
        'legalRepresentative',
        'registeredCapital',
        'establishmentDate',
        'companyAddress',
        'businessScope'
      ],
      // 字段 key -> 中文名（工商校验差异提示文案用）
      fieldLabelMap: {
        supplierName: '供应商名称',
        unifiedCreditCode: '企业统一社会信用代码',
        legalRepresentative: '法定代表人',
        registeredCapital: '注册资本',
        establishmentDate: '成立日期',
        companyAddress: '公司地址',
        businessScope: '经营范围'
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
    },
    // 统一社会信用代码校验规则：必填（onblur 触发）
    creditCodeRules() {
      return [
        { required: true, message: '请输入统一社会信用代码', trigger: 'blur' }
      ];
    },
    // 供应商联系方式校验规则：必填 + 11位手机号码（onblur 触发）
    contactPhoneRules() {
      return [
        { required: true, message: '请输入供应商联系方式', trigger: 'blur' }
        // { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的11位手机号码', trigger: 'blur' }
      ];
    },
    // 供应商名称是否禁用：台账入口（携带台账 id）锁定不可切换；审批态/校验成功后同样锁定
    isSupplierNameDisabled() {
      return !!this.defaultLedgerId || this.isBusinessFieldDisabled('supplierName');
    },
    // 工商校验状态标签（供应商名称下拉后侧展示）
    verifyStatusTag() {
      const map = {
        unverified: { text: '未校验', type: 'info' },
        success: { text: '校验成功', type: 'success' },
        skip: { text: '已入库', type: 'warning' },
        fail: { text: '校验失败', type: 'danger' }
      };
      return map[this.formModel.verifyStatus] || map.unverified;
    },
    // 被修改字段清单（变更对比列表与变更明细的共用数据源）；
    // 数组归一化为分隔串比对，避免对象数组 String() 后均为 "[object Object]" 被误判相等
    // 按类型拆分的附件（三类字段分别回显用）：
    // 严格按文件数据的 attachmentType 匹配（1/2/3），未关联类型的文件不展示
    attachmentsByType() {
      const result = { 1: [], 2: [], 3: [] };
      const list = this.formModel.attachments || [];
      list.forEach(file => {
        if (result[file.attachmentType]) {
          result[file.attachmentType].push(file);
        }
      });
      return result;
    },
    // 附件是否全空（全空且可编辑时在考察报告附件下方提示至少上传一种类型）
    attachmentsEmpty() {
      return !(this.formModel.attachments || []).length;
    },
    changedKeyValues() {
      if (!this.originalData) return [];
      const list = [];
      const normalize = (key, v) => {
        if (v === undefined || v === null) return '';
        if (Array.isArray(v)) {
          // 附件按"文件id+类型"归一化：文件在类型间移动、同名替换（id 已变）都能被识别为变更
          if (key === 'attachments') {
            return v.map(f => `${this.attachmentFileKey(f)}[${(f && f.attachmentType) || ''}]`).join(',');
          }
          return this.formatArrayValue(v, ',');
        }
        return String(v);
      };
      Object.keys(this.compareFieldMap).forEach(key => {
        const oldVal = this.originalData[key];
        const newVal = this.formModel[key];
        if (normalize(key, oldVal) !== normalize(key, newVal)) {
          list.push({ key, oldVal, newVal });
        }
      });
      return list;
    },
    // 变更对比列表（表单下方展示）：中文字段名 + 展示值（品类转名称、空值显示 '<空>'）；
    // 附件变更按 attachmentType 拆分展示（企业介绍附件/经营业绩附件/考察报告附件），
    // 仅展示真正发生变化的附件类型（该类型文件 id 清单与原值不一致才出条目，展示值为文件名清单）
    changedFields() {
      const rows = [];
      this.changedKeyValues.forEach(({ key, oldVal, newVal }) => {
        if (key === 'attachments') {
          this.attachmentFields.forEach(t => {
            const oldFiles = this.filterFilesByType(oldVal, t.value);
            const newFiles = this.filterFilesByType(newVal, t.value);
            // 按文件 id 清单判断该类型是否变更（同名替换 id 已变也能识别），展示值仍为文件名清单
            const oldIds = oldFiles.map(f => this.attachmentFileKey(f)).join(',');
            const newIds = newFiles.map(f => this.attachmentFileKey(f)).join(',');
            if (oldIds === newIds) return;
            rows.push({
              key: `attachments_${t.value}`,
              label: t.label,
              oldValue: this.displayFileNames(oldFiles),
              newValue: this.displayFileNames(newFiles)
            });
          });
          return;
        }
        rows.push({
          key,
          label: this.compareFieldMap[key],
          oldValue: this.displayCompareValue(key, oldVal),
          newValue: this.displayCompareValue(key, newVal)
        });
      });
      return rows;
    },
    // 变更明细：字段名驼峰转下划线，oldValue/newValue 直取快照与表单的字段值（保持真实类型）；
    // 大类/二级品类 id 变更时追加名称变更条目（旧值取台账自带名称，新值按当前选项解析）
    changeDetails() {
      const list = [];
      this.changedKeyValues.forEach(({ key, oldVal, newVal }) => {
        list.push({
          fieldName: camelToSnake(key),
          oldValue: oldVal,
          newValue: newVal
        });
        if (key === 'categoryId') {
          list.push({
            fieldName: 'category_name',
            oldValue: this.originalData.categoryName,
            newValue: this.resolveCategoryName(newVal)
          });
        } else if (key === 'subCategoryId') {
          list.push({
            fieldName: 'sub_category_name',
            oldValue: this.originalData.subCategoryName,
            newValue: this.resolveSubCategoryName(newVal)
          });
        }
      });
      return list;
    },
    // 变更内容展示列表：发起态（有快照）用实时比对结果，审批/回显态用随表单数据保存的列表
    displayChangedFields() {
      return this.originalData ? this.changedFields : this.savedChangedFields;
    }
  },
  watch: {
    // 一级大类变化时联动加载二级品类选项
    'formModel.categoryId'() {
      this.loadSecondCategory();
    },
    // 外部值变化（回显/草稿恢复）时重新装载表单模型与已保存的变更内容；与当前一致时跳过，避免回环
    value(val) {
      const parsed = safeParseModel(val);
      const savedFields = extractSavedChangedFields(val);
      const sameModel = JSON.stringify(parsed) === JSON.stringify(this.formModel);
      const sameFields = JSON.stringify(savedFields) === JSON.stringify(this.savedChangedFields);
      if (sameModel && sameFields) return;
      if (!sameModel) this.formModel = parsed;
      if (!sameFields) this.savedChangedFields = savedFields;
    },
    // 数据变化同步回 form-making；名称/信用代码均未填时发空字符串，保证设计器必填校验能拦截空数据
    formModel: {
      handler(model) {
        this.emitComponentData();
      },
      deep: true
    },
    // 仅变更明细变化而表单数据未变时（如草稿回显后原始快照重建），补发一次同步
    changedFields() {
      this.emitComponentData();
    }
  },
  created() {
    // 台账"申请变更"入口：读取 $initFlow 注入的台账 id，待下拉选项就绪后默认选中
    this.defaultLedgerId = (this.initFlowDialogOptions?.params?.ledgerId) || '';
    this.loadFirstCategory();
    this.loadLedgerList();
    // 审批/打印态归属公司反显数据源
    this.loadCompanyListForView();
  },
  mounted() { },
  methods: {
    // 一级大类列表：供应商中心分类接口
    loadFirstCategory() {
      this.$axios.post(
        Api.supplierCenter.categoryList,
        { data: {}},
        (res) => {
          if (!res.isSuccess) return;
          this.firstCategoryList = res.data || [];
          // 已选一级大类时，列表就绪后补一次二级品类加载（回显场景）
          this.loadSecondCategory();
          // 大类选项就绪（categoryId 按名称反查兜底的前提），尝试台账默认选中/重建原始快照
          this.tryApplyDefaultLedger();
          this.restoreOriginalSnapshot();
        }
      );
    },

    // 二级品类选项：按归属公司 + 一级大类组合查询（categoryId + companyId），条件缺失时清空选项；
    // 归属公司优先取表单数据里的 companyId（审批态跨公司回显时，按查看者公司查询的选项
    // 不含表单已选品类，二级品类会裸显 id），兜底当前用户所选公司
    loadSecondCategory() {
      const categoryId = this.formModel.categoryId;
      const companyId = this.formModel.companyId || localstorageGet('companyId');
      if (!categoryId || !companyId) {
        this.secondCategoryOptions = [];
        return;
      }
      this.$axios.post(
        Api.supplierCenter.subCategoryList,
        { data: { categoryId, companyId }},
        (res) => {
          if (!res.isSuccess) return;
          this.secondCategoryOptions = res.data || [];
        }
      );
    },

    // 查询当前用户所属公司的在册供应商台账（与清退组件同接口同条件），作为供应商名称下拉选项
    loadLedgerList() {
      // 审批态不可编辑，无需拉取
      if (this.disabled) return;
      this.$axios.post(
        Api.supplierCenter.getSupplierList,
        { data: { companyId: localstorageGet('companyId'), ledgerStatus: 1, current: 1, size: 9999 }},
        (res) => {
          if (!res.isSuccess) return;
          this.ledgerOptions = (res.data && res.data.records) || [];
          this.tryApplyDefaultLedger();
          // 草稿回显场景：按已选供应商从台账选项重建原始快照，恢复变更对比能力
          this.restoreOriginalSnapshot();
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

    // 台账入口默认选中：台账与大类选项均就绪后执行，仅新发起（表单无值）时生效
    tryApplyDefaultLedger() {
      if (this.defaultLedgerApplied || !this.defaultLedgerId) return;
      if (!this.ledgerOptions.length || !this.firstCategoryList.length) return;
      this.defaultLedgerApplied = true;
      if (this.formModel.supplierName || this.formModel.unifiedCreditCode) return;
      const ledger = this.ledgerOptions.find(item => item.id === this.defaultLedgerId);
      if (!ledger) {
        this.$message.warning('未在台账中找到指定供应商，请手动选择');
        return;
      }
      this.applyLedgerData(ledger);
    },

    // 选择/切换供应商：按台账记录带入该供应商数据
    handleSupplierChange(supplierName) {
      const ledger = this.ledgerOptions.find(item => item.supplierName === supplierName);
      if (!ledger) return;
      this.applyLedgerData(ledger);
    },

    // 台账记录 -> 规范化供应商数据（带入与原始快照共用；列表接口未返回的字段留空待补填）
    buildLedgerData(ledger) {
      const categoryId = ledger.categoryId ||
        (this.firstCategoryList.find(item => item.name === ledger.categoryName) || {}).id ||
        '';
      return {
        ledgerId: ledger.id || '',
        supplierCode: ledger.supplierCode || '',
        supplierName: ledger.supplierName || '',
        companyId: localstorageGet('companyId'),
        unifiedCreditCode: ledger.unifiedCreditCode || '',
        companyAddress: ledger.companyAddress || '',
        supplierLevel: ledger.supplierLevel || '',
        cooperationType: ledger.cooperationType || '',
        categoryId,
        subCategoryId: ledger.subCategoryId || '',
        registeredCapital: ledger.registeredCapital || '',
        establishmentDate: ledger.establishmentDate || '',
        legalRepresentative: ledger.legalRepresentative || '',
        qualificationLevel: ledger.qualificationLevel || '',
        mainQualificationName: ledger.mainQualificationName || '',
        mainQualificationValidity: ledger.mainQualificationValidity || '',
        safetyLicenseNo: ledger.safetyLicenseNo || '',
        safetyLicenseValidity: ledger.safetyLicenseValidity || '',
        personnelQualification: ledger.personnelQualification || '',
        threeSystemCert: ledger.threeSystemCert || '',
        creditStatus: ledger.creditStatus || '',
        performanceStatus: ledger.performanceStatus || '',
        contactPerson: ledger.contactPerson || '',
        contactPhone: ledger.contactPhone || '',
        businessScope: ledger.businessScope || '',
        remark: ledger.remark || ''
      };
    },

    // 解析台账原始附件：attachments 为文件数组的 JSON 字符串（或数组），空/异常兜底空数组
    parseLedgerAttachments(ledger) {
      const raw = ledger && ledger.attachments;
      if (!raw) return [];
      try {
        const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw;
        return Array.isArray(parsed) ? parsed : [];
      } catch (error) {
        return [];
      }
    },

    // 台账记录带入表单并保存原始快照；切换供应商时重置校验状态与变更原因，
    // 附件按台账原始数据解析带入（随 attachmentType 分类型回显）
    applyLedgerData(ledger) {
      const data = this.buildLedgerData(ledger);
      Object.assign(this.formModel, data, {
        // —— 切换供应商重置 ——
        verifyStatus: 'unverified',
        diffFields: [],
        attachments: this.parseLedgerAttachments(ledger),
        changeReason: ''
      });
      this.originalData = this.buildSnapshot(ledger);
    },

    // 原始快照：比对字段 + 台账自带的大类/品类名称（名称变更条目旧值直取）；
    // 附件基线取台账原始附件（与表单带入一致），未增删/移动类型时不产生附件变更条目
    buildSnapshot(ledger) {
      const data = this.buildLedgerData(ledger);
      data.attachments = this.parseLedgerAttachments(ledger);
      const snapshot = this.pickCompareFields(data);
      snapshot.categoryName = ledger.categoryName || '';
      snapshot.subCategoryName = ledger.subCategoryName || '';
      return snapshot;
    },

    // 摘取对比字段作为快照：空值归一，数组克隆避免与表单共用引用
    pickCompareFields(source) {
      const snapshot = {};
      Object.keys(this.compareFieldMap).forEach(key => {
        const val = source[key];
        snapshot[key] = Array.isArray(val) ? val.slice() : (val === undefined || val === null ? '' : val);
      });
      return snapshot;
    },

    // 草稿回显场景：按已选供应商从台账选项重建原始快照（已保存过则跳过）
    restoreOriginalSnapshot() {
      if (this.originalData || !this.formModel.supplierName) return;
      const ledger = this.ledgerOptions.find(item => item.supplierName === this.formModel.supplierName);
      if (!ledger) return;
      this.originalData = this.buildSnapshot(ledger);
    },

    // 一级大类 id -> 名称（未匹配返回 id 兜底）
    resolveCategoryName(val) {
      if (val === undefined || val === null || val === '') return '';
      const hit = this.firstCategoryList.find(item => item.id === val);
      return hit ? hit.name : val;
    },

    // 二级品类 id -> 名称（未匹配返回 id 兜底）
    resolveSubCategoryName(val) {
      if (val === undefined || val === null || val === '') return '';
      const hit = this.secondCategoryOptions.find(item => item.id === val);
      return hit ? hit.name : val;
    },

    // 数组值格式化：对象数组（附件，EleUpload 结构取 fileName）取文件名，字符串数组（附件类型）取自身，按分隔符连接
    formatArrayValue(list, separator) {
      return (list || [])
        .map(item => (item && typeof item === 'object' && (item.fileName || item.name) !== undefined ? (item.fileName || item.name) : String(item)))
        .join(separator);
    },

    // 变更对比展示值：大类/二级品类转名称展示，数组字段（附件类型/附件上传）按分隔符连接，空值显示 '<空>'
    displayCompareValue(key, val) {
      if (Array.isArray(val)) {
        const text = this.formatArrayValue(val, '、');
        return text || '<空>';
      }
      if (val === undefined || val === null || val === '') return '<空>';
      if (key === 'categoryId') {
        return this.resolveCategoryName(val);
      }
      if (key === 'subCategoryId') {
        // 旧值名称从原供应商数据取：切换大类后二级品类选项已替换为新大类的，旧品类 id 无法反查
        if (this.originalData && val === this.originalData.subCategoryId && this.originalData.subCategoryName) {
          return this.originalData.subCategoryName;
        }
        return this.resolveSubCategoryName(val);
      }
      return val;
    },

    // 对外数据：台账id + 供应商编码 + 表单全量字段 + 变更明细（后端格式）+ 变更内容列表（展示格式，供回显）
    getComponentData() {
      return Object.assign({}, this.formModel, {
        changeDetails: this.changeDetails,
        changedFields: this.changedFields
      });
    },

    // 同步对外数据到 form-making；与当前值一致时跳过，避免回环
    emitComponentData() {
      const model = this.formModel;
      const payload = model.supplierName || model.unifiedCreditCode ? JSON.stringify(this.getComponentData()) : '';
      if (payload !== this.value) {
        this.$emit('input', payload);
      }
    },

    // 一级大类切换：清空已选二级品类
    handleFirstCategoryChange() {
      this.formModel.subCategoryId = '';
    },

    // 工商信息字段是否禁用：审批态禁用；工商校验成功后工商信息字段禁用编辑，防止校验过的数据被改错
    isBusinessFieldDisabled(key) {
      if (this.disabled) return true;
      return !!(this.formModel.verifyStatus === 'success' && this.verifiedLockKeys.indexOf(key) > -1);
    },

    // 工商数据校验：单条调批量校验接口（dataList 单条）；
    // 已校验的比对企查查工商信息并自动更新差异字段，已跳过的标记为已入库
    handleVerifySupplier() {
      const supplier = this.formModel;
      if (!supplier.supplierName && !supplier.unifiedCreditCode) {
        this.$message.warning('请先填写供应商名称或统一社会信用代码');
        return;
      }
      const params = {
        companyId: localstorageGet('companyId'),
        // 变更流程的工商信息校验--需要后台取消对供应商是否已入库的判断，否则校验结果永远是已入库
        businessType: 'supplier_change_approval',
        dataList: [{
          supplierName: supplier.supplierName,
          unifiedCreditCode: supplier.unifiedCreditCode
        }]
      };
      this.$axios.post(
        Api.supplierCenter.batchVerify,
        { data: params },
        (res) => {
          if (!res.isSuccess) {
            this.$message.error(res.message || '校验接口返回错误');
            return;
          }
          const resultList = (res.data && res.data.data) || [];
          const hit = resultList.find(v => v.unifiedCreditCode === supplier.unifiedCreditCode);
          if (!hit) {
            this.$message.error('校验接口未返回该供应商信息');
            return;
          };
          if (hit.verifyStatus === '已校验') {
            if (hit.businessData) {
              this.applyBusinessData(supplier, hit.businessData);
              // 自动写入的字段重跑一次校验：有值即通过，清除此前空值失焦留下的残留报错
              this.revalidateFilledFields(supplier.diffFields);
            }
            // 差异字段转中文列名，用于确认框文案
            const diffLabels = (supplier.diffFields || []).map(key => this.fieldLabelMap[key] || key);
            const finishVerify = () => {
              this.$message.success('校验成功');
              supplier.verifyStatus = 'success';
            };
            if (diffLabels.length) {
              this.$confirm(`根据工商数据查询结果，${diffLabels.join('、')}信息存在差异，现已将该部分信息自动更新为工商查询结果`, '提示', {
                type: 'warning',
                showCancelButton: false,
                showClose: false,
                closeOnPressEscape: false,
                closeOnClickModal: false
              })
                .then(finishVerify)
                .catch(() => {});
            } else {
              finishVerify();
            }
          } else if (hit.verifyStatus === '已跳过') {
            supplier.verifyStatus = 'skip';
            this.$message.warning('当前供应商已入库，无需再次入库');
          } else {
            supplier.verifyStatus = 'fail';
            this.$message.error('校验失败，请检查供应商名称或统一社会信用代码是否正确');
          }
        }
      );
    },

    // 工商信息回填：校验成功后统一替换为查询结果（空/未定义的字段赋 '-'），并记录差异字段
    applyBusinessData(item, businessData) {
      const diffFields = [];
      Object.keys(this.businessFieldMap).forEach(bizKey => {
        const field = this.businessFieldMap[bizKey];
        const newVal = (businessData[bizKey] === undefined || businessData[bizKey] === null || businessData[bizKey] === '')
          ? '-'
          : businessData[bizKey];
        if (String(item[field] || '') !== String(newVal)) {
          item[field] = newVal;
          diffFields.push(field);
        }
      });
      item.diffFields = diffFields;
    },

    // 工商校验自动写入后，对被写入字段重跑校验（有值即通过，清除残留报错）
    revalidateFilledFields(fields) {
      if (!fields || !fields.length) return;
      this.$nextTick(() => {
        if (!this.$refs.supplierForm) return;
        fields.forEach(field => this.$refs.supplierForm.validateField(field, () => {}));
      });
    },

    // 从附件数组中筛选指定类型的文件（严格按 attachmentType 匹配）
    filterFilesByType(list, type) {
      return (list || []).filter(file => file.attachmentType === type);
    },
    // 附件文件的对比键：文件 id（fileId 优先，与 EleUpload getFileId 同口径），无 id 兜底空串
    attachmentFileKey(file) {
      return String((file && (file.fileId || file.id)) || '');
    },
    // 附件文件名展示值：多个文件名顿号连接，空显示 '<空>'
    displayFileNames(files) {
      const text = (files || []).map(f => (f && f.fileName) || '').filter(Boolean).join('、');
      return text || '<空>';
    },
    // 分类型附件校验规则：该分类（attachmentType）下无已上传文件则拦截，message 带类型名称
    typedAttachmentRules(field) {
      if (!field.required) return [];
      return [
        {
          required: true,
          validator: (rule, value, callback) => {
            const list = this.formModel.attachments || [];
            const hasFile = list.some(file => file.attachmentType === field.value);
            callback(hasFile ? undefined : new Error(`请上传${field.label}`));
          },
          trigger: 'change'
        }
      ];
    },

    // 分类型附件列表变化（EleUpload @change，含上传成功/删除）：
    // 该类型的文件以 EleUpload 回调列表为准（补上 attachmentType 落库），其他类型的文件保持不变
    handleTypedAttachmentsChange(type, list) {
      const typedFiles = (list || [])
        .filter(item => item.status === 'uploaded')
        .map(item => ({
          fileName: item.name,
          fileUrl: item.fileUrl,
          absolutelyFileUrl: item.absolutelyFileUrl,
          fileId: item.fileId,
          id: item.id,
          fileType: item.fileType,
          attachmentType: type
        }));
      const otherFiles = this.formModel.attachments.filter(file => file.attachmentType !== type);
      this.formModel.attachments = otherFiles.concat(typedFiles);
    }
  }
};
</script>
<style lang='scss' scoped>
.supplier-change .supplier-form {
  ::v-deep {
    .el-textarea__inner {
      font-size: 14px !important;
    }
  }
}
.supplier-change {
  width: 100%;
  background: #fff;
  padding: 16px 20px;
  box-sizing: border-box;

  // 变更表单区
  .supplier-form {
    min-width: 0;
    overflow: auto;
    border: 1px solid #ebeef5;
    border-radius: 8px;
    padding: 12px;

    // 表单网格：普通字段每行 2 列，多行文本整行
    .form-grid {
      display: flex;
      flex-wrap: wrap;

      .form-item {
        flex: 0 0 50%;
        box-sizing: border-box;
        padding: 0 10px;

        &.full-row {
          flex: 0 0 100%;
        }

        // 附件全空提示：灰色小字，与表单内容左对齐（label 宽 160px + 右留白 12px）
        .attachment-required-tip {
          margin-left: 160px;
          font-size: 12px;
          color: #909399;
          text-align: left;
        }

        // 日期/下拉控件占满格宽
        .form-control {
          width: 100%;
        }
      }
    }

    // —— 抵消 form-making 内置样式
    .form-grid ::v-deep .el-form-item__label {
      width: 160px !important; // 与 el-form 的 label-width="160px" 保持一致
      padding: 0 12px 0 0 !important;
      text-align: right !important;
      font-size: 14px !important;
    }

    .form-grid ::v-deep .el-form-item__content {
      width: auto !important;
      text-align: left !important;
      margin-bottom: 14px !important;
    }

    .form-grid ::v-deep .el-input__inner {
      text-align: left !important;
      font-size: 14px !important;
    }

    // 外层 form-making 字段校验失败时，.el-form-item.is-error 的后代选择器会把组件内
    // 所有输入框染红（含非必填、已填写的字段）；此处仅还原"自身未报错"的表单项边框
    .form-grid ::v-deep .el-form-item:not(.is-error) {
      .el-input .el-input__inner {
        border-color: #DCDFE6;          // Element 默认边框色
      }

      .el-input.is-disabled .el-input__inner {
        border-color: #E4E7ED;          // 保持禁用态样式（归属公司等）
      }

      .el-textarea .el-textarea__inner {
        border-color: #DCDFE6;
      }
    }

    // 变更内容对比面板：栅格外独立容器，宽度固定为表单宽（overflow:hidden 兜底，
    // 保证容器宽度不随表格内容增长，掐断 el-table 的重测量拉伸循环）
    .change-panel {
      width: 100%;
      margin-top: 4px;
      padding: 0 10px;
      padding-left: 170px;
      box-sizing: border-box;
      overflow: hidden;
      margin-bottom: 20px;

      .change-table-container {
        padding: 5px 0;
        border: 1px solid #ebeef5;
        border-radius: 8px;
      }

      .change-panel-title {
        font-size: 14px;
        line-height: 32px;
        text-align: left;
      }
    }

    // 变更原因行（栅格外）：套用与栅格内表单项一致的 label/内容样式
    .change-reason-row {
      padding: 0 10px;
      box-sizing: border-box;

      ::v-deep .el-form-item__label {
        width: 160px !important;
        padding: 0 12px 0 0 !important;
        text-align: right !important;
        font-size: 14px !important;
      }

      ::v-deep .el-form-item__content {
        width: auto !important;
        text-align: left !important;
        margin-bottom: 14px !important;
      }

      ::v-deep .el-input__inner {
        text-align: left !important;
        font-size: 14px !important;
      }
    }

    // 调试按钮行：与表单内容区左边距对齐
    .debug-row {
      padding: 0 10px 6px 170px;
      text-align: left;
    }
  }
}
</style>
