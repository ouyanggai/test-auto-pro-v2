/*
 * @Descripttion:自定义过滤器
 * @Author: Calvin
 * @Date: 2021-05-20 10:42:55
 */

const filters = {
  /**
   *
   * @param val {Number, String}
   * @param num {Number, String}
   * @param retain {Number}
   * @return {String}
   */
  qualityCheckStatus: (value) => {
    if (value == 'not_submit') {
      return '未提交';
    } else if (value == 'submit') {
      return '提交';
    } else if (value == 'reject') {
      return '驳回';
    } else if (value == 'end') {
      return '结束';
    }
  },
  /**
   *
   * @param val {Number, String}
   * @param num {Number, String}
   * @param retain {Number}
   * @return {String}
   */
  planState: (value) => {
    if (value == 'under_review') {
      return '进行中';
    } else if (value == 'end' || value == 'finish') {
      return '已完成';
    } else if (value == 'not_enabled' || value == 'enabled') {
      return '未开始';
    }
  },
  /**
   *质量数据检验状态
   * @param val {Number, String}
   * @param num {Number, String}
   * @param retain {Number}
   * @return {String}
   */
  executeResult: (value) => {
    if (value == 'submit') {
      return '待检验';
    } else if (value == 'end') {
      return '通过';
    } else if (value == 'reject') {
      return '未通过';
    } else {
      return '--';
    }
  },
  /**
   *获取流程节点名称
   *
   * @return {String}
   */
  filtersFlowNodes: (arr) => {
    if (!arr.length) {
      return '';
    }
    const result = [];
    arr.forEach(item => {
      result.push(item.nodeName);
    });

    return result.join('→');
  },
  translateStatus: (type) => {
    let result;
    switch (type) {
      case 'await_sent':
        result = '待发';
        break;

      case 'run':
        result = '运行中';
        break;

      case 'withdraw':
        result = '撤销';
        break;

      case 'termination':
        result = '终止';
        break;

      case 'rejected':
        result = '驳回';
        break;

      case 'end':
        result = '完结';
        break;

      default:
        result = '';
        break;
    }
    return result;
  },
  workTargetType(type) {
    let result;
    switch (type) {
      case '1':
        result = '待发';
        break;
      case '2':
        result = '运行中';
        break;
      default:
        result = '';
        break;
    }
    return result;
  },
  manageTargetType(type) {
    let result;
    switch (type) {
      case '1':
        result = '制度和流程';
        break;
      case '2':
        result = '执行与协作';
        break;
      case '3':
        result = '个人发展与成长';
        break;

      case '4':
        result = '客户维护';
        break;
      case '5':
        result = '安全与保密';
        break;
      case '6':
        result = '费用控制';
        break;
      case '7':
        result = '团队建设';
        break;
      case '8':
        result = '发展与成长';
        break;
      case '9':
        result = '客户维护与开发';
        break;
      case '10':
        result = '企业文化的理解与践行';
        break;
      case '11':
        result = '企业文化的贯彻与践行';
        break;
      default:
        result = '';
        break;
    }
    return result;
  }
};

export default filters;
