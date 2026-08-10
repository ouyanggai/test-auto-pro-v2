import processInstance from '@/layout/components/globalDialog/process.js';
class Fm {
  vm;
  visible;
  currentComponent;
  propData = {};
  constructor(vm) {
    this.vm = vm;
    this.visible = false;
    this.process = processInstance;
  }

  async show(component, propData) {
    // console.log('component-自定义组件',component)
    // console.log('propData-自定义组件',propData)
    this.propData = propData;
    this.currentComponent = component;
    this.visible = true;
    var instance;
    await this.vm.$nextTick(() => {
      instance = this.vm.$refs[this.currentComponent];
    });
    console.log('instance-自定义组件',instance)
    window.$vue.testInstance = instance;
    return instance;
  }

  show2(component, propData) {
    this.propData = propData;
    this.currentComponent = component;
    this.visible = true;
    return new Promise((resolve) => {
      this.vm.$nextTick(() => {
        var instance = this.vm.$refs[this.currentComponent];
        instance.$on('confirmed', res => resolve(res));
      });
    });
  }

  close() {
    this.visible = false;
  }
};

export default Fm;
