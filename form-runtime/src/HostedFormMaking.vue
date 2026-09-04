<template>
  <OtherSteps2
    ref="host"
    :flow-id="''"
    :form-id="''"
    :steps="2"
    :select-flow-type="''"
    :company-id="companyId"
  />
</template>

<script>
import OtherSteps2 from '@runtime/views/GroupApproveManage/Submitted/components/OtherSteps2.vue'

// clonePlain 在 iframe 桥接边界复制数据，避免把父页面或 FormMaking 的响应式代理传入目标宿主。
function clonePlain (value) {
  return JSON.parse(JSON.stringify(value == null ? {} : value))
}

export default {
  name: 'HostedFormMaking',
  components: { OtherSteps2 },
  props: {
    companyId: {
      type: String,
      default: ''
    }
  },
  data () {
    return {
      boundForm: null,
      changeHandler: null
    }
  },
  beforeDestroy () {
    this.unbindChange()
  },
  methods: {
    host () {
      return this.$refs.host || null
    },
    form () {
      const host = this.host()
      return host && host.$refs && host.$refs.generateForm ? host.$refs.generateForm : null
    },
    // bindChange 复用目标宿主的 FormMaking 事件，不在桥接层复制组件事件和字段规则。
    bindChange () {
      const form = this.form()
      if (!form || typeof form.$on !== 'function' || this.boundForm === form) return
      this.unbindChange()
      this.boundForm = form
      this.changeHandler = () => this.$emit('change')
      form.$on('on-change', this.changeHandler)
    },
    unbindChange () {
      if (this.boundForm && this.changeHandler && typeof this.boundForm.$off === 'function') {
        this.boundForm.$off('on-change', this.changeHandler)
      }
      this.boundForm = null
      this.changeHandler = null
    },
    // load 只负责把工作区已经核验的完整模板、权限和有效值交给目标宿主，不改写业务字段。
    async load (template, values, permissions = {}) {
      const host = this.host()
      if (!host) throw new Error('目标表单宿主尚未完成装载')
      host.enableData = Array.isArray(permissions.editableFields) ? [...permissions.editableFields] : []
      host.disabledData = Array.isArray(permissions.disabledFields) ? [...permissions.disabledFields] : []
      host.jsonData = clonePlain(template)
      host.editData = clonePlain(values)
      await this.$nextTick()
      this.bindChange()
      const form = this.form()
      if (!form || typeof form.setData !== 'function') throw new Error('目标 FormMaking 运行时缺少 setData 能力')
      await form.setData(clonePlain(values))
      return form
    },
    // setData 同步宿主 editData 与 FormMaking，保证附件区和字段区看到同一份回放值。
    async setData (values) {
      const host = this.host()
      if (host) host.editData = clonePlain(values)
      const form = this.form()
      if (!form || typeof form.setData !== 'function') throw new Error('目标 FormMaking 运行时缺少 setData 能力')
      await form.setData(clonePlain(values))
      this.bindChange()
      return form
    }
  }
}
</script>
