<!--
 * @Descripttion: 供应商批量入库申请
 * @Author: junshao
-->
<template>
  <div class="supplier-entry">
    <!-- 顶部：居中标题 + 下方操作按钮（下载模板/导入数据，审批态隐藏） -->
    <div class="ledger-header">
      <div v-if="!disabled" class="header-actions">
        <el-button type="primary" plain icon="el-icon-download" @click="handleDownloadTemplate">下载模板</el-button>
        <el-button type="primary" icon="el-icon-upload2" @click="handleOpenUploadDialog">导入数据</el-button>
      </div>
    </div>

    <!-- 页面主体：左侧供应商列表 + 右侧表单信息 -->
    <div class="page-body">
      <div class="supplier-aside">
        <div class="aside-title">
          供应商列表（{{ supplierList.length }}）
          <div v-if="!disabled" class="aside-tip">供应商入库单次不能超过10条数据</div>
        </div>
        <div v-if="!disabled && supplierList.length < 10" class="aside-toolbar">
          <el-button type="primary" plain size="small" icon="el-icon-plus" class="add-supplier-btn" data-testid="6ec92e87-e2cf-4161-9e69-c46702866751" @click="handleAddSupplier">新增供应商</el-button>
        </div>
        <div class="aside-list">
          <div v-for="(item, index) in supplierList" :key="index" class="aside-item" :class="{ active: index === activeIndex }"
            @click="handleSelectSupplier(index)">
            <div class="aside-row">
              <span class="aside-index">{{ index + 1 }}</span>
              <span class="aside-name" :title="item.supplierName">{{ item.supplierName || '-' }}</span>
              <i v-if="!disabled" class="el-icon-delete remove-icon" @click.stop="handleRemoveSupplier(index)"></i>
            </div>
            <el-tag v-if="!disabled" :type="verifyStatusTagMap[item.verifyStatus] || 'info'" size="mini" class="aside-status">{{ verifyStatusTextMap[item.verifyStatus] || '未校验' }}</el-tag>
          </div>
          <!-- 空状态 -->
          <div v-if="!supplierList.length" class="aside-empty">暂无数据</div>
        </div>
      </div>

      <!-- 右侧：所选供应商的表单信息（Excel 全部字段，两列分布） -->
      <div class="supplier-form">
        <el-form v-if="activeSupplier" ref="supplierForm" :model="activeSupplier" label-position="right" label-width="160px" size="small">
          <div class="form-grid">
            <div class="form-item">
              <el-form-item label="供应商名称" prop="supplierName" :rules="[{ required: true, message: '请输入供应商名称', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.supplierName" data-testid="133499a5-0596-4625-acd5-70b0029c2fd5" :disabled="isBusinessFieldDisabled('supplierName')" placeholder="请输入" maxlength="25" show-word-limit></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="归属公司" prop="companyId" :rules="[{ required: true, message: '请选择归属公司', trigger: 'change' }]">
                <el-select v-model="activeSupplier.companyId" data-testid="f0e79042-c25a-4c78-bbc0-e177e446e589" disabled placeholder="请选择" class="form-control">
                  <el-option v-for="opt in companyOptions" :key="opt.value" :label="opt.label" :value="opt.value"></el-option>
                </el-select>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="统一社会信用代码" prop="unifiedCreditCode" :rules="creditCodeRules">
                <div style="display: flex;">
                  <el-input v-model="activeSupplier.unifiedCreditCode" data-testid="6785c0f6-87f9-4c0b-9597-777ef7f972c6" :disabled="isBusinessFieldDisabled('unifiedCreditCode')" placeholder="请输入" style="flex: 1;"></el-input>
                  <el-button v-if="!disabled" type="primary" data-testid="19a37843-df39-46a5-b8f9-91bc5376ed2e" @click="handleVerifySupplier" style="margin-left: 10px;width: 100px;">工商数据校验</el-button>
                </div>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="公司地址" prop="companyAddress" :rules="[{ required: true, message: '请输入公司地址', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.companyAddress" data-testid="18f6c6f5-3151-448c-ab7a-291d2ae013a7" :disabled="isBusinessFieldDisabled('companyAddress')" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="级别" prop="supplierLevel" :rules="[{ required: true, message: '请选择级别', trigger: 'change' }]">
                <el-select v-model="activeSupplier.supplierLevel" data-testid="b68a7232-b4ae-4442-ab9a-1cfe91ecb5fa" :disabled="disabled" placeholder="请选择" class="form-control">
                  <el-option v-for="opt in levelOptions" :key="opt" :label="opt" :value="opt"></el-option>
                </el-select>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="合作类型" prop="cooperationType" :rules="[{ required: true, message: '请选择合作类型', trigger: 'change' }]">
                <el-select v-model="activeSupplier.cooperationType" data-testid="4f45ea76-97dc-4acf-9d5e-6283c526b39d" :disabled="disabled" placeholder="请选择" class="form-control">
                  <el-option v-for="opt in cooperationTypeOptions" :key="opt" :label="opt" :value="opt"></el-option>
                </el-select>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="一级大类" prop="categoryId" :rules="[{ required: true, message: '请选择一级大类', trigger: 'change' }]">
                <el-select v-model="activeSupplier.categoryId" data-testid="5b48ccd5-0f6e-47c9-b56e-0c3a1f91ecea" :disabled="disabled" placeholder="请选择" class="form-control" @change="handleFirstCategoryChange">
                  <el-option v-for="opt in firstCategoryList" :key="opt.id" :label="opt.name" :value="opt.id"></el-option>
                </el-select>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="二级品类" prop="subCategoryId" :rules="[{ required: true, message: '请选择二级品类', trigger: 'change' }]">
                <el-select v-model="activeSupplier.subCategoryId" data-testid="a70c745d-3893-4e44-bf31-bb47a52b4b66" :disabled="disabled" placeholder="请选择" class="form-control">
                  <el-option v-for="opt in secondCategoryOptions" :key="opt.id" :label="opt.name" :value="opt.id"></el-option>
                </el-select>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="注册资本" prop="registeredCapital" :rules="[{ required: true, message: '请输入注册资本', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.registeredCapital" data-testid="80813d43-2b97-4d78-ac7a-13dbdfe138f0" :disabled="isBusinessFieldDisabled('registeredCapital')" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="成立日期" prop="establishmentDate" :rules="[{ required: true, message: '请选择成立日期', trigger: 'change' }]">
                <el-date-picker v-model="activeSupplier.establishmentDate" data-testid="49cfc6f6-02f9-4bb1-9cd2-dbda45542d42" :disabled="isBusinessFieldDisabled('establishmentDate')" type="date" value-format="yyyy-MM-dd" placeholder="请选择日期"
                  class="form-control"></el-date-picker>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="法定代表人" prop="legalRepresentative" :rules="[{ required: true, message: '请输入法定代表人', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.legalRepresentative" data-testid="5f104c7d-3bbb-45d1-ace3-4203548f33e9" :disabled="isBusinessFieldDisabled('legalRepresentative')" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <!-- <el-form-item label="资质等级" prop="qualificationLevel" :rules="[{ required: true, message: '请选择资质等级', trigger: 'change' }]"> -->
              <el-form-item label="资质等级" prop="qualificationLevel">
                <el-select v-model="activeSupplier.qualificationLevel" data-testid="11b335cb-2c00-4d73-85b3-7bd87969db89" :disabled="disabled" placeholder="请选择" class="form-control">
                  <el-option v-for="opt in qualificationLevelOptions" :key="opt" :label="opt" :value="opt"></el-option>
                </el-select>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="主体资质名称" prop="mainQualificationName">
                <el-input v-model="activeSupplier.mainQualificationName" data-testid="2ec10fd9-83d7-4641-a1c0-fef7c6e13183" :disabled="disabled" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="主体资质有效期" prop="mainQualificationValidity">
                <el-date-picker v-model="activeSupplier.mainQualificationValidity" data-testid="db53f60d-2b9f-4de4-ad2d-8c642076c81e" :disabled="disabled" type="date" value-format="yyyy-MM-dd" :picker-options="validityPickerOptions" placeholder="请选择日期"
                  class="form-control"></el-date-picker>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="安全生产许可证编号" prop="safetyLicenseNo">
                <el-input v-model="activeSupplier.safetyLicenseNo" data-testid="ecf71856-36ff-43de-8011-3035e1a11079" :disabled="disabled" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="安全生产许可证有效期" prop="safetyLicenseValidity">
                <el-date-picker v-model="activeSupplier.safetyLicenseValidity" data-testid="1f31a6dc-a6d6-459c-a572-0a47dd28ed8e" :disabled="disabled" type="date" value-format="yyyy-MM-dd" :picker-options="validityPickerOptions" placeholder="请选择日期"
                  class="form-control"></el-date-picker>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="人员资质情况" prop="personnelQualification">
                <el-input v-model="activeSupplier.personnelQualification" data-testid="8af9f9d7-8328-47d3-93bf-a95c61df153a" :disabled="disabled" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="三标体系认证" prop="threeSystemCert">
                <el-select v-model="activeSupplier.threeSystemCert" data-testid="939a0d88-fdb1-437b-9f83-2ce85db98e53" :disabled="disabled" placeholder="请选择" class="form-control">
                  <el-option label="是" value="是"></el-option>
                  <el-option label="否" value="否"></el-option>
                </el-select>
              </el-form-item>
            </div>
            <div class="form-item full-row">
              <el-form-item label="信用情况" prop="creditStatus" :rules="[{ required: true, message: '请输入信用情况', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.creditStatus" data-testid="98909f2b-cb99-4db3-985b-7c5c8eb80dd2" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <div class="form-item full-row">
              <el-form-item label="业绩情况" prop="performanceStatus" :rules="[{ required: true, message: '请输入业绩情况', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.performanceStatus" data-testid="8b442d83-e09b-42ed-9a8d-bf16c5763a4d" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="供应商联系人" prop="contactPerson" :rules="[{ required: true, message: '请输入供应商联系人', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.contactPerson" data-testid="a2d69528-29e4-4655-b6e8-5ae3f706a194" :disabled="disabled" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <div class="form-item">
              <el-form-item label="供应商联系方式" prop="contactPhone" :rules="contactPhoneRules">
                <el-input v-model="activeSupplier.contactPhone" data-testid="922f32fb-b0c1-4c77-b0e7-4a16e8810590" :disabled="disabled" placeholder="请输入" maxlength="11"></el-input>
              </el-form-item>
            </div>
            <div class="form-item full-row">
              <el-form-item label="经营范围" prop="businessScope" :rules="[{ required: true, message: '请输入经营范围', trigger: 'blur' }]">
                <el-input v-model="activeSupplier.businessScope" data-testid="feebb42e-0f79-43e1-b8ab-5dcd6d91e42f" :disabled="isBusinessFieldDisabled('businessScope')" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
            <!-- 附件全空提示：仅在可编辑态展示 -->
            <!-- <div v-if="!disabled && attachmentsEmpty" class="form-item full-row">
              <div class="attachment-required-tip"><i class="el-icon-info"></i> 至少选择一种类型的附件进行上传</div>
            </div> -->
            <!-- 附件按类型区分：三种类型各带上传按钮（均必填），文件统一存 attachments（带 attachmentType 属性）；回显态隐藏按钮仅展示 -->
            <div class="form-item full-row" v-for="t in attachmentFields" :key="t.value">
              <el-form-item :label="t.label" prop="attachments" :rules="typedAttachmentRules(t)">
                <eleUpload :attachFile="attachmentsByType[t.value]" :data-testid="attachmentTestIdMap[t.value]" :isDuplicateOverwrite="false" :size="300" :uploadLimit="20" :showOnly="disabled" @change="list => handleTypedAttachmentsChange(t.value, list)"></eleUpload>
              </el-form-item>
            </div>
            <div class="form-item full-row">
              <el-form-item label="备注" prop="remark">
                <el-input v-model="activeSupplier.remark" data-testid="c538b628-0f96-42f4-9586-b4cb1224e529" :disabled="disabled" type="textarea" :autosize="{ minRows: 2}" placeholder="请输入"></el-input>
              </el-form-item>
            </div>
          </div>
        </el-form>
        <!-- 无数据时的占位 -->
        <div v-else class="form-empty">请先点击左侧「导入数据」或「新增供应商」添加数据</div>
      </div>
    </div>

    <!-- 导入过渡弹窗：上传引导，选择文件解析成功后进入预览弹窗 -->
    <el-dialog title="Excel 批量导入" :visible.sync="uploadDialogVisible" width="500px" :close-on-click-modal="false" append-to-body>
      <div class="upload-guide">
        <i class="el-icon-upload upload-icon"></i>
        <div class="upload-title">上传Excel文件</div>
        <div class="upload-tip">支持.xlsx/.xls格式，单次最多支持10条数据，文件大小不超过2MB</div>
        <el-upload ref="upload" action="#" accept=".xlsx,.xls" :limit="1" :file-list="fileList" :show-file-list="false"
          :http-request="handleUploadFile">
          <el-button type="primary">选择文件</el-button>
        </el-upload>
      </div>
    </el-dialog>

    <!-- 导入数据预览弹窗：解析结果 + 数据预览（工商数据校验）+ 校验结果统计 -->
    <el-dialog title="供应商批量入库" :visible.sync="dialogVisible" :width="dialogWidth" top="12vh" :close-on-click-modal="false" append-to-body @close="handleDialogClosed">
      <div class="dialog-body">
        <!-- 解析结果 -->
        <div class="parse-result">
          <div><i class="el-icon-document" style="color: #52c41a;font-size: 18px;"></i> {{ importFileName }}</div>
          <div style="color: #52c41a;">
            共解析 {{ parseSummary.total }} 条数据，
            <span v-if="parseSummary.missingNameAndCode > 0">{{ parseSummary.missingNameAndCode }} 条数据缺失名称和信用代码，</span>
            <span v-if="parseSummary.duplicate > 0">重复数据 {{ parseSummary.duplicate }} 条，</span>
            实际导入 {{ parsedList.length }} 条数据</div>
        </div>
        <!-- 数据预览标题 + 工商数据校验按钮 -->
        <div class="preview-toolbar">
          <span class="preview-title">数据预览</span>
          <el-button type="primary" size="small" icon="el-icon-circle-check" @click="handleVerifyBusinessData">工商数据校验</el-button>
        </div>
        <el-table :data="parsedList" border stripe size="medium" style="width: 100%;" max-height="460px">
          <!-- 首列序号自增 -->
          <el-table-column type="index" label="序号" width="60" align="center"></el-table-column>
          <el-table-column v-for="field in previewColumns" :key="field.key" :prop="field.key" :label="field.label" :min-width="field.minWidth"
            show-overflow-tooltip>
            <template slot-scope="{ row }">
              <!-- 工商校验自动更新的差异字段用蓝色字体标识 -->
              <span :class="{ 'diff-value': row.diffFields && row.diffFields.includes(field.key) }">{{ row[field.key] || '-' }}</span>
            </template>
          </el-table-column>
          <!-- 末列校验状态 -->
          <el-table-column label="校验状态" width="110" align="center">
            <template slot-scope="{ row }">
              <el-tag :type="verifyStatusTagMap[row.verifyStatus] || 'info'" size="small">{{ verifyStatusTextMap[row.verifyStatus] || '未校验' }}</el-tag>
            </template>
          </el-table-column>
          <!-- 空状态 -->
          <template slot="empty">
            <div class="empty-block">
              <img class="empty-img" :src="noDataImg" alt="暂无数据" />
              <div class="empty-text">暂无数据</div>
            </div>
          </template>
        </el-table>
        <!-- 校验结果统计 -->
        <div class="verify-result">
          <div>
            <span style="color: #52c41a;">已校验【{{ verifiedCount }}】条数据，</span>
            <span>【{{ unverifiedCount }}】条数据待校验</span>
          </div>
          <!-- 提示信息 -->
          <div v-if="skippedSupplierNames.length" class="verify-line">
            <span style="color: #e8a641;">*{{ skippedSupplierNames.join('*、*') }}*</span>
            <span>供应商已经入库，如需修改请在入库台账中点击【申请变更】</span>
          </div>
          <div v-if="hasDiffFields" class="verify-line"><span class="diff-text">蓝色字体</span>的工商信息和企查查工商信息查询结果存在差异，已根据查询结果为您自动更新</div>
          <div v-if="hasSkippedOrFailed" class="verify-line" style="color: #ff4949;">点击确认导入时，*校验失败*及*已入库*的供应商将被自动跳过</div>
        </div>
      </div>

      <div slot="footer" class="dialog-footer">
        <el-button type="primary" @click="handleConfirmImport">确 认</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script>
import XLSX from '@/lib/xlsx/xlsx.full.min.js';
import noDataImg from '@/assets/images/no_data.png';
import Api from '@/api';
import eleUpload from '@/components/EleUpload';
import { localstorageGet } from '@/utils/auth';

// 解析 form-making 传入的 value（JSON 字符串 -> 供应商数组），空值/异常兜底为空数组
function safeParseList(val) {
  if (!val) return [];
  try {
    const parsed = JSON.parse(val);
    return Array.isArray(parsed) ? parsed : [];
  } catch (error) {
    return [];
  }
}

// 导入解析与工商回填需标准化为 yyyy-MM-dd 的日期字段（无法识别的格式直接置空，避免写坏表单日期组件）
const DATE_FIELD_KEYS = ['establishmentDate', 'mainQualificationValidity', 'safetyLicenseValidity'];

export default {
  name: 'SupplierEntry',
  components: { eleUpload },
  props: {
    // form-making v-model 通道：JSON.stringify(supplierList)，空列表为空字符串
    value: {
      type: String,
      default: ''
    },
    // 审批态：控件全部禁用，隐藏导入/新增/删除入口
    disabled: {
      type: Boolean,
      default: false
    },
    // 打印态：只读表格汇总
    printRead: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      noDataImg,
      // 页面展示的供应商数据
      supplierList: safeParseList(this.value),
      activeIndex: 0,
      fileList: [],
      parsing: false,
      uploadDialogVisible: false,
      dialogVisible: false,
      importFileName: '',
      // 解析结果统计（原始解析条数 / 名称与信用代码同时缺失条数 / 重复跳过条数）
      parseSummary: {
        total: 0,
        missingNameAndCode: 0,
        duplicate: 0
      },
      parsedList: [],
      levelOptions: ['一级', '二级', '三级'],
      // 合作类型下拉选项
      cooperationTypeOptions: ['供应商', '施工商', '服务商', '其他类'],
      firstCategoryList: [],
      secondCategoryOptions: [],
      // 审批/打印态归属公司反显用：全量公司列表（发起态不加载）
      allCompanyList: [],
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
        1: 'ea8a2dd2-d5b6-4910-8a5e-e4c2ead171d0',
        2: '70b107f0-bd28-40ab-b2e1-717238209974',
        3: '9202aa1e-d335-480f-bdb3-6e3865bfd9fe'
      },
      // 附件类型字段（按类型区分上传；文件统一存 attachments，带 attachmentType 属性；required 控制是否必填）
      attachmentFields: [
        { label: '企业介绍附件', value: 1, required: true },
        { label: '经营业绩附件', value: 2, required: true },
        { label: '考察报告附件', value: 3, required: true }
      ],
      // Excel 模板列配置（key = 表单字段名，label = 模板表头，顺序与模板列序一致）
      excelColumns: [
        { key: 'supplierName', label: '供应商名称' },
        { key: 'unifiedCreditCode', label: '企业统一社会信用代码' },
        { key: 'legalRepresentative', label: '法定代表人' },
        { key: 'registeredCapital', label: '注册资本' },
        { key: 'establishmentDate', label: '成立日期' },
        { key: 'companyAddress', label: '公司地址' },
        { key: 'businessScope', label: '经营范围' },
        { key: 'cooperationType', label: '合作类型' },
        // { key: 'companyId', label: '归属公司' },
        { key: 'categoryId', label: '一级大类' },
        { key: 'subCategoryId', label: '二级品类' },
        { key: 'supplierLevel', label: '级别' },
        { key: 'qualificationLevel', label: '资质等级' },
        { key: 'mainQualificationName', label: '主体资质名称' },
        { key: 'mainQualificationValidity', label: '主体资质有效期' },
        { key: 'safetyLicenseNo', label: '安全生产许可证编号' },
        { key: 'safetyLicenseValidity', label: '安全生产许可证有效期' },
        { key: 'personnelQualification', label: '人员资质情况' },
        { key: 'threeSystemCert', label: '三标体系认证' },
        { key: 'creditStatus', label: '信用情况' },
        { key: 'performanceStatus', label: '业绩情况' },
        { key: 'contactPerson', label: '供应商联系人' },
        { key: 'contactPhone', label: '供应商联系方式' },
        { key: 'remark', label: '备注' }
      ],
      // 预览表格展示列（仅工商信息相关字段，列宽按内容设置）
      previewColumns: [
        { key: 'supplierName', label: '供应商名称', minWidth: 180 },
        { key: 'unifiedCreditCode', label: '企业统一社会信用代码', minWidth: 180 },
        { key: 'legalRepresentative', label: '法定代表人', minWidth: 120 },
        { key: 'registeredCapital', label: '注册资本', minWidth: 110 },
        { key: 'establishmentDate', label: '成立日期', minWidth: 110 },
        { key: 'companyAddress', label: '公司地址', minWidth: 180 },
        { key: 'businessScope', label: '经营范围', minWidth: 220 }
      ],
      // 校验状态码 -> 展示文案
      verifyStatusTextMap: {
        unverified: '未校验',
        success: '校验成功',
        fail: '校验失败',
        skip: '已入库'
      },
      // 校验状态码 -> el-tag 颜色类型
      verifyStatusTagMap: {
        unverified: 'info',
        success: 'success',
        fail: 'danger',
        skip: 'warning'
      },
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
      // 已入库（校验跳过）的供应商信用代码列表
      skippedCreditCodes: []
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
    // 当前选中的供应商（右侧表单直接绑定其字段）
    activeSupplier() {
      return this.supplierList[this.activeIndex] || null;
    },
    // 统一社会信用代码校验规则：必填 + 与列表内其他供应商查重（onblur 触发）
    creditCodeRules() {
      return [
        { required: true, message: '请输入统一社会信用代码', trigger: 'change' },
        { validator: this.validateCreditCode, trigger: 'blur' }
      ];
    },
    // 供应商联系方式校验规则：必填 + 11位手机号码（onblur 触发）
    contactPhoneRules() {
      return [
        { required: true, message: '请输入供应商联系方式', trigger: 'blur' }
        // { pattern: /^1[3-9]\d{9}$/, message: '请输入正确的11位手机号码', trigger: 'blur' }
      ];
    },
    // 工商校验成功后禁用编辑的字段：以 previewColumns 为准
    verifiedLockKeys() {
      return this.previewColumns.map(col => col.key);
    },
    // 按类型拆分的附件（三类字段分别回显用）：
    // 严格按文件数据的 attachmentType 匹配（1/2/3），未关联类型的文件不展示
    attachmentsByType() {
      const result = { 1: [], 2: [], 3: [] };
      const list = (this.activeSupplier && this.activeSupplier.attachments) || [];
      list.forEach(file => {
        if (result[file.attachmentType]) {
          result[file.attachmentType].push(file);
        }
      });
      return result;
    },
    // 附件是否全空（全空且可编辑时在考察报告附件下方提示至少上传一种类型）
    attachmentsEmpty() {
      return !((this.activeSupplier && this.activeSupplier.attachments) || []).length;
    },
    // 已校验条数（校验成功/已入库/校验失败均视为已处理）
    verifiedCount() {
      return this.parsedList.filter(item => item.verifyStatus !== 'unverified').length;
    },
    // 未校验条数
    unverifiedCount() {
      return this.parsedList.length - this.verifiedCount;
    },
    // 已入库的供应商名称列表（按信用代码匹配）
    skippedSupplierNames() {
      return this.parsedList
        .filter(item => this.skippedCreditCodes.indexOf(item.unifiedCreditCode) > -1)
        .map(item => item.supplierName);
    },
    // 是否存在被工商校验自动更新的差异字段
    hasDiffFields() {
      return this.parsedList.some(item => item.diffFields && item.diffFields.length);
    },
    // 是否存在确认导入时会被跳过的供应商（校验失败/已入库）
    hasSkippedOrFailed() {
      return this.parsedList.some(item => item.verifyStatus === 'skip' || item.verifyStatus === 'fail');
    },
    // 弹窗宽度：窗口宽度的 70%
    dialogWidth() {
      return `${window.innerWidth * 0.7}px`;
    }
  },
  watch: {
    // （切换供应商使当前表单一级大类值变化时同样会触发）
    'activeSupplier.categoryId'() {
      this.loadSecondCategory();
    },
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
    this.loadFirstCategory();
    // 审批/打印态归属公司反显数据源
    this.loadCompanyListForView();
  },
  mounted() { },
  methods: {
    // 一级大类列表：供应商中心分类接口；返回 Promise 供确认导入转换时兜底复用
    loadFirstCategory() {
      return this.$axios.post(
        Api.supplierCenter.categoryList,
        { data: {}},
        (res) => {
          if (!res.isSuccess) return;
          this.firstCategoryList = res.data || [];
          // 当前表单已选一级大类时，列表就绪后补一次二级品类加载（草稿回显场景）
          this.loadSecondCategory();
        }
      );
    },

    // 二级品类选项：按归属公司 + 一级大类组合查询（categoryId + companyId），条件缺失时清空选项；
    // 归属公司优先取表单数据里的 companyId（审批态跨公司回显时，按查看者公司查询的选项
    // 不含表单已选品类，二级品类会裸显 id），兜底当前用户所选公司
    loadSecondCategory() {
      const categoryId = this.activeSupplier ? this.activeSupplier.categoryId : '';
      const companyId = (this.activeSupplier && this.activeSupplier.companyId) || localstorageGet('companyId');
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

    // 日期值标准化：宽容解析 yyyy-M-d / yyyy/M/d / yyyy.M.d / yyyy年M月d日 / yyyyMMdd 等常见写法，
    // 统一为 yyyy-MM-dd；无法识别或非真实日期（如 2024/13/40）返回空字符串
    normalizeDateValue(val) {
      const str = String(val == null ? '' : val).trim();
      if (!str) return '';
      const m = str.match(/^(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})$/)
        || str.match(/^(\d{4})年(\d{1,2})月(\d{1,2})日?$/)
        || str.match(/^(\d{4})(\d{2})(\d{2})$/);
      if (!m) return '';
      const year = Number(m[1]);
      const month = Number(m[2]);
      const day = Number(m[3]);
      const date = new Date(year, month - 1, day);
      if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) return '';
      return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
    },

    // 打开导入过渡弹窗（真正的文件选择在弹窗内）
    handleOpenUploadDialog() {
      this.uploadDialogVisible = true;
    },

    // 自定义上传：仅本地解析 Excel，不发网络请求
    handleUploadFile(param) {
      const file = param.file;
      // 文件类型校验
      if (!file.name.match(/\.(xlsx|xls)$/i)) {
        this.$message.error('请上传Excel文件（.xlsx或.xls格式）');
        return;
      }
      // 文件大小校验：正常模板仅几十 KB，超过 2M 视为文件异常
      if (file.size > 2 * 1024 * 1024) {
        this.$message.warning('文件大小不能超过2M');
        return;
      }
      // 重置待确认数据与 el-upload，允许连续导入
      this.parsedList = [];
      this.dialogVisible = false;
      this.$nextTick(() => {
        if (this.$refs.upload) {
          this.$refs.upload.clearFiles();
        }
      });

      this.parsing = true;
      const reader = new FileReader();
      reader.onload = (e) => {
        try {
          const data = e.target.result;
          // dateNF：内置日期格式14是区域敏感格式，库按字面 m/d/yy 渲染会与中文Excel显示(yyyy/m/d)不一致，
          // 此处对齐为中文Excel短日期显示，实现"Excel里显示什么就解析出什么"（自定义格式/文本列不受影响）
          const workbook = XLSX.read(data, { type: 'array', dateNF: 'yyyy/m/d' });
          // 取第一个可见工作表，转二维数组（第一行为表头）
          // 模板含隐藏数据表 _data_（下拉数据源），SheetNames[0] 可能取到隐藏表，需按 Hidden 标记跳过
          const firstSheetName = workbook.SheetNames.find(name => {
            const sheetIdx = workbook.SheetNames.indexOf(name);
            const sheetInfo = workbook.Workbook && workbook.Workbook.Sheets[sheetIdx];
            return !sheetInfo || !sheetInfo.Hidden;
          }) || workbook.SheetNames[0];
          const worksheet = workbook.Sheets[firstSheetName];
          const jsonData = XLSX.utils.sheet_to_json(worksheet, {
            header: 1,
            defval: '',
            raw: false
          });

          if (!jsonData.length) {
            this.$message.warning('解析Excel文件失败，请使用正确的模板文件');
            return;
          }

          // 校验表头与模板一致（trim 后逐列全等才通过；不校验数据内容）
          const headers = jsonData[0].map(h => String(h == null ? '' : h).trim());
          const headerMatched = this.excelColumns.every((f, i) => headers[i] === f.label);
          if (!headerMatched) {
            this.$message.warning('文件导入失败，请点击下载模板，使用正确的模板文件进行导入');
            return;
          }

          // 过滤整行空行，按列序映射为表单字段对象（模板列 key 与表单字段一一对应）
          const dataRows = jsonData.slice(1).filter(row =>
            row.some(cell => cell !== '' && cell !== null && cell !== undefined)
          );
          this.parsedList = dataRows.map(row => {
            const supplier = {};
            this.excelColumns.forEach((f, i) => {
              const raw = row[i] || '';
              // 日期字段标准化为 yyyy-MM-dd，无法识别的格式置空（Excel 填写格式不可控）
              supplier[f.key] = DATE_FIELD_KEYS.indexOf(f.key) > -1 ? this.normalizeDateValue(raw) : raw;
            });
            // 模板外的页面字段补空值
            supplier.attachments = [];
            supplier.remark = '';
            // 重置归属公司为当前登录用户所属公司
            supplier.companyId = localstorageGet('companyId');
            // 导入数据默认未校验；diffFields 记录工商校验自动更新的差异字段（表格蓝色显示）
            supplier.verifyStatus = 'unverified';
            supplier.diffFields = [];
            return supplier;
          });

          if (!this.parsedList.length) {
            this.$message.warning('Excel中没有可导入的数据');
            return;
          }
          // 单文件最多支持 10 条数据（按清洗后的数据量校验）
          if (this.parsedList.length > 10) {
            this.$message.warning('单文件最多支持10条数据，请调整文件后重新导入');
            return;
          }

          // 数据预处理：剔除名称与信用代码同时缺失的数据，并按企业信用代码去重（重复只保留一条）
          const { list, missing, duplicate } = this.preprocessParsedList(this.parsedList);
          this.parseSummary = {
            total: this.parsedList.length,
            missingNameAndCode: missing,
            duplicate
          };
          this.parsedList = list;

          if (!this.parsedList.length) {
            this.$message.warning('导入的数据均缺失供应商名称和统一社会信用代码，请检查文件后重新导入');
            return;
          }

          this.importFileName = file.name;
          this.dialogVisible = true;
          this.$nextTick(() => {
            this.uploadDialogVisible = false;
          });
        } catch (error) {
          console.error('解析Excel文件失败:', error);
          this.$message.error('解析Excel文件失败，请使用正确的模板文件');
        } finally {
          this.parsing = false;
        }
      };
      reader.onerror = () => {
        this.$message.error('读取文件失败，请重试');
        this.parsing = false;
      };
      reader.readAsArrayBuffer(file);
    },

    // 数据预处理：供应商名称与统一社会信用代码同时缺失的数据剔除；有信用代码的按信用代码去重（重复只保留第一条）
    // 返回清洗后的数组与统计（缺失条数、重复跳过条数——同一代码出现 N 次累计 N-1）
    preprocessParsedList(list) {
      const creditCodeSet = new Set();
      const result = [];
      let missing = 0;
      let duplicate = 0;
      list.forEach(item => {
        const supplierName = String(item.supplierName || '').trim();
        const unifiedCreditCode = String(item.unifiedCreditCode || '').trim();
        // 名称与统一社会信用代码同时缺失：无法导入，直接剔除
        if (!supplierName && !unifiedCreditCode) {
          missing += 1;
          return;
        }
        // 仅统一社会信用代码参与去重；有名称无代码的数据保留且不参与去重
        if (unifiedCreditCode) {
          if (creditCodeSet.has(unifiedCreditCode)) {
            duplicate += 1;
            return;
          }
          creditCodeSet.add(unifiedCreditCode);
        }
        result.push(item);
      });
      return { list: result, missing, duplicate };
    },

    // 清除表单校验状态（仅新增空行时使用：数据尚未填写，不立即报错）
    clearFormValidate() {
      this.$nextTick(() => {
        if (this.$refs.supplierForm) this.$refs.supplierForm.clearValidate();
      });
    },

    // 清除遗留校验状态后按当前行数据重新校验（切换/删除后使用）
    refreshFormValidate() {
      this.$nextTick(() => {
        if (!this.$refs.supplierForm) return;
        this.$refs.supplierForm.clearValidate();
        this.$refs.supplierForm.validate(() => {});
      });
    },

    // 左侧列表点击切换
    handleSelectSupplier(index) {
      this.activeIndex = index;
      this.refreshFormValidate();
    },

    // 删除供应商：确认后从列表中移除，并修正选中项下标
    handleRemoveSupplier(index) {
      const supplierName = this.supplierList[index].supplierName || '-';
      this.$confirm(`确认要删除供应商【${supplierName}】及其入库信息吗？`, '提示', { type: 'warning' })
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
          this.refreshFormValidate();
        })
        .catch(() => {});
    },

    // 下载模板
    handleDownloadTemplate() {
      this.$axios.post(
        Api.supplierCenter.getExcelTemplate,
        {
          data: {
            companyId: localstorageGet('companyId')
          }
        },
        res => {
          if (res) {
            const blob = new Blob([res], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
            const link = document.createElement('a');
            link.style.display = 'none';
            link.href = URL.createObjectURL(blob);
            link.download = '供应商入库申请导入模板.xlsx';
            link.click();
            window.URL.revokeObjectURL(link.href);
            link.remove();
          } else { this.$message.error('下载模板失败'); }
        }, '', { responseType: 'blob' }
      )
        .then(res => {
          const blob = res.data;
          const a = document.createElement('a');
          a.href = URL.createObjectURL(blob);
          a.download = '供应商批量入库申请模板.xlsx';
          a.click();
          URL.revokeObjectURL(a.href);
        });
    },

    // 统一社会信用代码查重：与左侧列表中其他供应商比对（排除当前编辑行自身，避免回显误报）
    validateCreditCode(rule, value, callback) {
      const val = String(value || '').trim();
      // 空值由必填规则提示，这里不重复报错
      if (!val) {
        callback();
        return;
      }
      const isDuplicate = this.supplierList.some((item, index) =>
        index !== this.activeIndex && String(item.unifiedCreditCode || '').trim() === val
      );
      callback(isDuplicate ? new Error('该统一社会信用代码在当前供应商列表中已存在') : undefined);
    },

    // 工商信息字段是否禁用：审批态禁用；供应商校验成功后 previewColumns 内字段禁用编辑，防止校验过的数据被改错
    isBusinessFieldDisabled(key) {
      if (this.disabled) return true;
      return !!(this.activeSupplier && this.activeSupplier.verifyStatus === 'success' && this.verifiedLockKeys.indexOf(key) > -1);
    },

    // 新增供应商：在列表最前面插入一条所有字段为空的数据，并选中它
    handleAddSupplier() {
      const emptySupplier = {};
      this.excelColumns.forEach(f => {
        emptySupplier[f.key] = '';
      });
      emptySupplier.supplierName = '';
      emptySupplier.companyId = localstorageGet('companyId');
      emptySupplier.attachments = [];
      emptySupplier.remark = '';
      emptySupplier.verifyStatus = 'unverified';
      emptySupplier.diffFields = [];
      this.supplierList.unshift(emptySupplier);
      this.activeIndex = 0;
      this.clearFormValidate();
    },

    // 一级大类切换：清空已选二级品类
    handleFirstCategoryChange() {
      if (this.activeSupplier) {
        this.activeSupplier.subCategoryId = '';
      }
    },

    // 分类型附件校验规则：该分类（attachmentType）下无已上传文件则拦截，message 带类型名称
    typedAttachmentRules(field) {
      if (!field.required) return [];
      return [
        {
          required: true,
          validator: (rule, value, callback) => {
            const list = (this.activeSupplier && this.activeSupplier.attachments) || [];
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
      if (!this.activeSupplier) return;
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
      const otherFiles = this.activeSupplier.attachments.filter(file => file.attachmentType !== type);
      this.activeSupplier.attachments = otherFiles.concat(typedFiles);
    },

    // 工商数据校验：调批量校验接口，按信用代码回填校验结果；
    // 已校验的比对企查查工商信息并自动更新差异字段（预览表格蓝色显示），已跳过的标记为已入库
    handleVerifyBusinessData() {
      const params = {
        companyId: localstorageGet('companyId'),
        dataList: this.parsedList.map(item => {
          return {
            supplierName: item.supplierName,
            unifiedCreditCode: item.unifiedCreditCode
          };
        })
      };
      this.$axios.post(
        Api.supplierCenter.batchVerify,
        { data: params },
        res => {
          if (!res.isSuccess) {
            this.$message.error(res.message || '校验接口返回错误');
            return;
          }
          const verificationResult = res.data.data || []; // 校验结果列表
          // 校验结果按信用代码索引，接口未返回的保持未校验
          const resultMap = {};
          verificationResult.forEach(v => {
            resultMap[v.unifiedCreditCode] = v;
          });
          // 已入库的供应商信用代码列表（统计区提示按此匹配供应商名称）
          this.skippedCreditCodes = res.data.skippedCreditCodes || [];
          this.parsedList.forEach(item => {
            const hit = resultMap[item.unifiedCreditCode];
            if (!hit) return;
            if (hit.verifyStatus === '已校验') {
              // 工商信息比对：差异字段更新为查询结果并标记
              if (hit.businessData) this.applyBusinessData(item, hit.businessData);
              item.verifyStatus = 'success';
            } else if (hit.verifyStatus === '已跳过') {
              item.verifyStatus = 'skip';
            } else {
              item.verifyStatus = 'fail';
            }
          });
        }
      );
    },

    // 工商信息回填：校验成功后统一替换为查询结果（空/未定义的字段赋 '-'），并记录差异字段（预览表格蓝色显示）；
    // 成立日期过标准化，无法识别或为空视为未返回、不覆盖——原逻辑空值会写 '-'，同样会写坏日期组件
    applyBusinessData(item, businessData) {
      const diffFields = [];
      Object.keys(this.businessFieldMap).forEach(bizKey => {
        const field = this.businessFieldMap[bizKey];
        const rawVal = businessData[bizKey];
        const newVal = field === 'establishmentDate'
          ? this.normalizeDateValue(rawVal)
          : (rawVal === undefined || rawVal === null || rawVal === '' ? '-' : rawVal);
        if (!newVal) return;
        if (String(item[field] || '') !== String(newVal)) {
          item[field] = newVal;
          diffFields.push(field);
        }
      });
      item.diffFields = diffFields;
    },

    // 工商校验自动写入后，对被写入字段重跑校验（有值即通过，清除残留报错）；
    // 异步等待期间用户已切到其他行时跳过（写入对象不再是当前表单行）
    revalidateFilledFields(fields, row) {
      if (!fields || !fields.length) return;
      this.$nextTick(() => {
        if (!this.$refs.supplierForm || this.activeSupplier !== row) return;
        fields.forEach(field => this.$refs.supplierForm.validateField(field, () => {}));
      });
    },

    // 表单区工商数据校验：对当前供应商单独调批量校验接口（dataList 单条）；
    handleVerifySupplier() {
      const supplier = this.activeSupplier;
      if (!supplier) return;
      if (!supplier.supplierName && !supplier.unifiedCreditCode) {
        this.$message.warning('请先填写供应商名称或统一社会信用代码');
        return;
      }
      // 校验前查重：供应商名称或统一社会信用代码与列表中其他供应商重复时，提示后终止校验（空值不参与比对）
      const name = String(supplier.supplierName || '').trim();
      const code = String(supplier.unifiedCreditCode || '').trim();
      const isNameDuplicated = this.supplierList.some((item, index) =>
        index !== this.activeIndex && name !== '' && String(item.supplierName || '').trim() === name
      );
      const isCodeDuplicated = this.supplierList.some((item, index) =>
        index !== this.activeIndex && code !== '' && String(item.unifiedCreditCode || '').trim() === code
      );
      if (isNameDuplicated || isCodeDuplicated) {
        const dupFields = [];
        if (isNameDuplicated) dupFields.push('供应商名称');
        if (isCodeDuplicated) dupFields.push('统一社会信用代码');
        this.$message.warning(`${dupFields.join('、')}与列表中其他供应商重复，请重新填写`);
        return;
      }
      const params = {
        companyId: localstorageGet('companyId'),
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
              this.revalidateFilledFields(supplier.diffFields, supplier);
            }
            // 差异字段转中文列名，用于确认框文案
            const diffLabels = (supplier.diffFields || []).map(key => {
              const col = this.excelColumns.find(f => f.key === key);
              return col ? col.label : key;
            });
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

    // 确认导入：仅导入校验成功的供应商；一/二级分类由 Excel 文本名称转换为 id 后落到页面
    handleConfirmImport() {
      const hasUnverified = this.parsedList.some(item => item.verifyStatus === 'unverified');
      if (hasUnverified) {
        this.$message.warning('导入数据前，请先完成工商数据校验');
        return;
      }
      const importableList = this.parsedList.filter(item => item.verifyStatus === 'success');
      if (!importableList.length) {
        this.$message.warning('暂无可导入的供应商数据');
        return;
      }
      this.convertCategoryFields(importableList)
        .then(() => {
          this.supplierList = importableList;
          this.activeIndex = 0;
          this.dialogVisible = false;
        });
    },

    // 一/二级分类名称转 id反显：Excel 导入的是文本名称，表单需要 id 绑定值
    convertCategoryFields(list) {
      // 一级列表正常在 created 已加载，此处兜底未就绪的情况
      const ready = this.firstCategoryList.length ? Promise.resolve() : this.loadFirstCategory();
      return ready.then(() => {
        // 一级：名称 -> id（匹配不上置空，二级随之失去归属也置空）
        list.forEach(item => {
          const hit = this.firstCategoryList.find(c => c.name === item.categoryId);
          item.categoryId = hit ? hit.id : '';
          if (!item.categoryId) item.subCategoryId = '';
        });
        // 二级：按 distinct 一级分类查询（归属公司固定为当前用户缓存），同一分类只查一次
        const companyId = localstorageGet('companyId');
        const categoryIds = [...new Set(list.filter(i => i.categoryId && i.subCategoryId).map(i => i.categoryId))];
        return Promise.all(categoryIds.map(categoryId =>
          this.$axios.post(Api.supplierCenter.subCategoryList, { data: { categoryId, companyId }})
            .then(res => res.data || [])
            .catch(() => [])
        )).then(results => {
          const subMap = {};
          categoryIds.forEach((id, i) => {
            subMap[id] = results[i];
          });
          list.forEach(item => {
            if (!item.categoryId || !item.subCategoryId) return;
            const hit = subMap[item.categoryId].find(s => s.name === item.subCategoryId);
            item.subCategoryId = hit ? hit.id : '';
          });
        });
      });
    },

    // 弹窗关闭（含右上角 X / ESC）：清空待确认数据
    handleDialogClosed() {
      this.parsedList = [];
      this.fileList = [];
    }
  }
};
</script>
<style lang='scss' scoped>
.supplier-entry .supplier-form {
  ::v-deep {
    .el-textarea__inner {
      font-size: 14px !important;
    }
  }
}
::v-deep {
  .el-table__row .el-table__cell {
    padding: 4px 0;
  }
}
// 表格空状态：暂无数据图 + 文案，居中
.empty-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 32px 0;

  .empty-img {
    width: 140px;
    height: auto;
  }

  .empty-text {
    margin-top: 8px;
    font-size: 14px;
    color: #909399;
  }
}
.supplier-entry {
  width: 100%;
  background: #fff;
  padding: 16px 20px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 16px;

  // 顶部：居中标题 + 下方操作按钮组
  .ledger-header {
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    align-items: center;

    // 标题下方操作按钮组
    .header-actions {
      display: flex;
      align-items: center;
    }
  }

  // 页面主体：左侧列表 + 右侧表单
  .page-body {
    flex: 1;
    min-height: 0;
    display: flex;
    gap: 16px;

    // 左侧供应商列表
    .supplier-aside {
      width: 18%;
      min-width: 240px;
      display: flex;
      flex-direction: column;
      border: 1px solid #ebeef5;
      border-radius: 8px;

      .aside-title {
        flex-shrink: 0;
        padding: 5px 12px;
        font-size: 14px;
        font-weight: 600;
        color: #303133;
        border-bottom: 1px solid #ebeef5;
        background: #f5f7fa;
        border-radius: 8px 8px 0 0;

        // 标题下方的单次数量上限提示（小字，仅填报态渲染）
        .aside-tip {
          margin-top: 2px;
          font-size: 12px;
          font-weight: 400;
          line-height: 16px;
          color: #909399;
        }
      }

      // 新增供应商按钮区
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
          flex-direction: column;
          align-items: flex-start;
          padding: 2px 5px;
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

          // 第一行：序号 + 名称 + 删除 icon 横向排列，占满整行
          .aside-row {
            display: flex;
            align-items: center;
            width: 100%;
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
            color: #ff1111;
            cursor: pointer;
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

// 弹窗内容：解析结果 + 数据预览 + 校验结果统计
.dialog-body {
  // 解析结果行
  .parse-result {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
    font-size: 14px;
    background-color: rgba(82, 196, 26, 0.1);
    border: 1px solid #52c41a;
    border-radius: 8px;
    padding: 12px 20px;
  }

  // 数据预览标题行：标题居左，校验按钮居右
  .preview-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;

    .preview-title {
      font-size: 14px;
      font-weight: 600;
      color: #303133;
    }
  }

  // 校验结果统计区（首行统计 + 条件展示的提示行）
  .verify-result {
    margin-top: 10px;
    font-size: 14px;
    background-color: rgba(150, 150, 150, 0.1);
    padding: 8px 20px;

    .verify-line {
      margin-top: 4px;
    }
  }

  // 工商校验自动更新的差异字段值（预览表格单元格）与提示行中的"蓝色字体"文案
  .diff-value,
  .diff-text {
    color: #1989fa;
  }
}

// 导入过渡弹窗：上传引导内容（图标 + 标题 + 提示 + 选择文件按钮，整体居中）
.upload-guide {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 20px 8px;

  .upload-icon {
    font-size: 48px;
    color: #1989fa;
  }

  .upload-title {
    margin-top: 12px;
    font-size: 16px;
    font-weight: 600;
    color: #303133;
  }

  .upload-tip {
    margin: 8px 0 20px;
    font-size: 14px;
    color: #909399;
  }
}

.dialog-footer {
  text-align: right;
}
</style>
