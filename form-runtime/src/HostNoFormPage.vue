<template>
  <section class="host-no-form-page">
    <h2>{{ pageTitle }}</h2>
    <div v-if="rows.length" class="host-no-form-page__table">
      <div v-for="row in rows" :key="row.path" class="host-no-form-page__row">
        <div class="host-no-form-page__label">{{ row.label }}</div>
        <div class="host-no-form-page__value">{{ row.value }}</div>
      </div>
    </div>
    <p v-else class="host-no-form-page__empty">暂无表单数据</p>
  </section>
</template>

<script>
const FIELD_LABELS = {
  userInfo: '用户姓名', userName: '姓名', companyName: '公司', departmentName: '部门', deptName: '部门',
  projectName: '项目名称', projectCode: '项目编号', contractName: '合同名称', contractNumber: '合同编号', year: '年度'
}

// flattenValues 将未知无表单页面的原始快照展平为可读字段，保证页面标识缺失时也能呈现业务数据。
function flattenValues (value, prefix, rows, depth) {
  if (depth > 4 || value === null || value === undefined) return
  if (Array.isArray(value)) {
    value.forEach((item, index) => flattenValues(item, `${prefix}[${index + 1}]`, rows, depth + 1))
    return
  }
  if (typeof value === 'object') {
    Object.keys(value).forEach(key => {
      if (key === 'id' || key.endsWith('Id') || key.endsWith('_id') || key === 'auto_audit_info') return
      const path = prefix ? `${prefix}.${key}` : key
      flattenValues(value[key], path, rows, depth + 1)
    })
    return
  }
  if (!prefix) return
  const key = prefix.split('.').pop().replace(/\[\d+\]$/, '')
  rows.push({ path: prefix, label: FIELD_LABELS[key] || '业务字段', value: String(value) })
}

export default {
  name: 'HostNoFormPage',
  props: {
    pageName: { type: String, default: '' },
    initialValues: { type: Object, default: () => ({}) },
    value: { type: Object, default: () => ({}) },
    data: { type: Object, default: () => ({}) }
  },
  computed: {
    pageTitle () { return this.pageName || '业务表单' },
    rows () {
      const rows = []
      const values = this.initialValues && Object.keys(this.initialValues).length ? this.initialValues : (this.value || this.data)
      flattenValues(values, '', rows, 0)
      return rows
    }
  },
  methods: {
    getValues () {
      const values = this.initialValues && Object.keys(this.initialValues).length
        ? this.initialValues
        : (this.value && Object.keys(this.value).length ? this.value : this.data)
      return { ...(values || {}) }
    }
  }
}
</script>

<style scoped>
.host-no-form-page { max-width: 1080px; margin: 0 auto; padding: 28px 32px 48px; color: #303133; }
.host-no-form-page h2 { margin: 0 0 24px; text-align: center; font-size: 24px; font-weight: 600; }
.host-no-form-page__table { border: 1px solid #dcdfe6; }
.host-no-form-page__row { display: grid; grid-template-columns: 180px minmax(0, 1fr); min-height: 44px; border-bottom: 1px solid #ebeef5; }
.host-no-form-page__row:last-child { border-bottom: 0; }
.host-no-form-page__label { padding: 11px 16px; background: #f5f7fa; border-right: 1px solid #ebeef5; font-weight: 500; }
.host-no-form-page__value { padding: 11px 16px; white-space: pre-wrap; word-break: break-word; }
.host-no-form-page__empty { color: #909399; text-align: center; }
@media (max-width: 640px) { .host-no-form-page { padding: 20px 12px 32px; } .host-no-form-page__row { grid-template-columns: 120px minmax(0, 1fr); } }
</style>
