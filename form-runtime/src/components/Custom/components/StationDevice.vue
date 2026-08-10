<template>
  <div>
    <el-dialog :visible="visible" :before-close="handleClose" width="650px" form :title="title" :action="false"
      append-to-body center @open="open">
      <div style="height: 70vh;overflow: auto;" class="wind-station-tree">
        <el-input placeholder="按设备名称、设备编码搜索查询" v-model="filterText" clearable size="mini"></el-input>
        <div v-loading="loading" style="margin-top: 15px;">
          <el-tree ref="tree" :filter-node-method="filterNode" node-key="id" :data="treeData" :props="defaultProps"
            :default-expand-all="false" show-checkbox :indent="10" auto-expand-parent check-on-click-node>
            <span slot-scope="{node,data}">
              <span>{{ data.name }}</span>
              <span style="color:#999999;margin-left: 19px;">{{ data.dutyName }}</span>
            </span>
          </el-tree>
        </div>
      </div>
      <span class="dialog-footer">
        <el-button size="mini" @click="handleClose">取 消</el-button>
        <el-button type="primary" size="mini" @click="confirm">确 定</el-button>
      </span>
    </el-dialog>
  </div>
</template>

<script>
/* eslint-disable */
import Api from '@/api/index.js';
export default {
  name: 'StationDevice',
  props: ['visible'],
  data() {
    return {
      loading: false,
      title: '选择设备',
      filterText: '',
      treeData: [],
      defaultProps: {
        children: 'children',
        label: 'name',
        disabled(data, node) {
          if (data.isDevice) {
            return false;
          } else {
            return true;
          }
        }
      }
    };
  },
  created() { },
  watch: {
    visible(val) {
      if (val) {
        this.initData();
      }
    },
    filterText(val) {
      this.$refs.tree.filter(val);
    }
  },
  computed: {},
  methods: {
    filterNode(value, data) {
      if (!value) return true;
      return data.name.indexOf(value) !== -1;
    },
    handleClose() {
      this.filterText = '';
      this.$emit('update:visible', false);
    },
    open(){
      this.filterText = '';
    },
    confirm() {
      const checkedDevice = [];
      const treeChecked = this.$refs.tree.getCheckedNodes();
      treeChecked.forEach(item => {
        if (item.isDevice) {
          checkedDevice.push(item);
        }
      });
      if (!checkedDevice.length) return this.$message.error('请选择设备');
      this.$emit('confirmChecked', checkedDevice);
    },
    getTree() {
      const data = {
        name: '',
        nodeId: ''
      };

      this.$axios.post(Api.equipLedger.getTree, { data }, res => {
        if (res.isSuccess) {
          const treeData = res.data;
          var deviceToTree = function (list, item) {
            list.forEach(el => {
              if (el.id) {
                if (el.id == item.deviceLegerLevelId) {
                  el.children.push(item);
                } else {
                  if (el?.children?.length) {
                    deviceToTree(el.children, item);
                  } else {

                  }
                }
              } else {
                const children = el.children;
                deviceToTree(children, item);
              }
            });
          };
          this.deviceLedger().then(resp => {
            this.loading = false;
            if (resp.isSuccess) {
              const dataList = resp.data.dataList;
              dataList.forEach(item => {
                item.isDevice = true;
                item.name = item.deviceName;
                deviceToTree(treeData, item);
              });
              this.treeData = treeData.filter(item=>item.children.length);
            }
          });
        }else{
          this.loading = false;
          this.treeData = [];
        }
      });
    },
    initData() {
      this.loading = true;
      // var treeData = []
      this.getTree();
    },
    deviceLedger() {
      return new Promise((resolve, reject) => {
        const data = {
          data: {
            searchKey: '',
            // deviceCategory: treedata?.name ?? '',
            deviceCategory: '',
            nodeId: '',
            startTime: '',
            endTime: ''
          },
          pagination: false
          // pages: this.pagination.pages,
          // size: this.pagination.size
        };
        this.$axios.post(Api.goodsLedger.deviceLedger, data, res => {
          resolve(res)
        });
      })
    }
  }
};
</script>
<style lang="scss" scoped>
.dialog-footer {
  display: block;
  text-align: center;
}

::v-deep .wind-station-tree .el-checkbox.is-disabled {
  display: none !important;
}
</style>
