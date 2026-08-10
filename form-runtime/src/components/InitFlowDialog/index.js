import Vue from 'vue';
import Dialog from './InitFlowDialog.vue';
import flowDetail from '@/layout/components/globalDialog/components/flowDetail.vue';
const DialogConstructor = Vue.extend(Dialog);
const FlowDetailConstructor = Vue.extend(flowDetail);
function createDialog(options = {}) {
  const div = document.createElement('div');
  document.body.appendChild(div);
  const instance = new DialogConstructor({
    parent: this,
    propsData: {
      title: options.title || '-',
      visible: true,
      options: options,
      params: options.params || {},
      callbacks: {
        onDataFetch: options.onDataFetch,
        onOpen: options.onOpen,
        onOpened: options.onOpened,
        onBeforeClose: options.onBeforeClose,
        onClose: options.onClose,
        onClosed: options.onClosed,
        onSuccess: options.onSuccess
      }
    }
  });
  instance.$mount(div);
  // instance.vm = instance.$mount();
  // document.body.appendChild(instance.vm.$el);
  return {
    instance,
    close() {
      instance.visible = false;
    },
    setData(data) {
      instance.dialogData = data;
    }
  };
}
function flowDetailDialog(options = {}) {
  const div = document.createElement('div');
  document.body.appendChild(div);
  const instance = new FlowDetailConstructor({
    parent: this,
    propsData: {
      dialogType: 'appendToBody',
      propData: {
        ...options,
        options: options,
        callback: options.callback || function() {}
      }
    }
  });
  instance.$mount(div);
  return {
    instance,
    close() {
      instance.visible = false;
    }
  };
}
Vue.prototype.$initFlow = createDialog;
Vue.prototype.$flowDetail = flowDetailDialog;
