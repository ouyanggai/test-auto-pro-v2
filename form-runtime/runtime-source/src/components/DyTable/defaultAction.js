
export function deleteDataAction(select) {
  this.$emit('delete', select);
}

export function filterAction() {
  console.log('filter', this);
  this.isadvance = !this.isadvance;
  return 0;
}
