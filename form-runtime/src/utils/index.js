/**
 * Created by PanJiaChen on 16/11/18.
 */
import { localstorageGet } from '@/utils/auth';
import { viewFileUrl } from '@/config/env.js';
import { Base64 } from 'js-base64';
/**
 * Parse the time to string
 * @param {(Object|string|number)} time
 * @param {string} cFormat
 * @returns {string | null}
 */
export function parseTime(time, cFormat) {
  if (arguments.length === 0 || !time) {
    return null;
  }
  const format = cFormat || '{y}-{m}-{d} {h}:{i}:{s}';
  let date;
  if (typeof time === 'object') {
    date = time;
  } else {
    if ((typeof time === 'string')) {
      if ((/^[0-9]+$/.test(time))) {
        // support "1548221490638"
        time = parseInt(time);
      } else {
        // support safari
        // https://stackoverflow.com/questions/4310953/invalid-date-in-safari
        time = time.replace(new RegExp(/-/gm), '/');
      }
    }

    if ((typeof time === 'number') && (time.toString().length === 10)) {
      time = time * 1000;
    }
    date = new Date(time);
  }
  const formatObj = {
    y: date.getFullYear(),
    m: date.getMonth() + 1,
    d: date.getDate(),
    h: date.getHours(),
    i: date.getMinutes(),
    s: date.getSeconds(),
    a: date.getDay()
  };
  const time_str = format.replace(/{([ymdhisa])+}/g, (result, key) => {
    const value = formatObj[key];
    // Note: getDay() returns 0 on Sunday
    if (key === 'a') { return ['日', '一', '二', '三', '四', '五', '六'][value]; }
    return value.toString().padStart(2, '0');
  });
  return time_str;
}

/**
 * @param {number} time
 * @param {string} option
 * @returns {string}
 */
export function formatTime(time, option) {
  if (('' + time).length === 10) {
    time = parseInt(time) * 1000;
  } else {
    time = +time;
  }
  const d = new Date(time);
  const now = Date.now();

  const diff = (now - d) / 1000;

  if (diff < 30) {
    return '刚刚';
  } else if (diff < 3600) {
    // less 1 hour
    return Math.ceil(diff / 60) + '分钟前';
  } else if (diff < 3600 * 24) {
    return Math.ceil(diff / 3600) + '小时前';
  } else if (diff < 3600 * 24 * 2) {
    return '1天前';
  }
  if (option) {
    return parseTime(time, option);
  } else {
    return (
      d.getMonth() +
      1 +
      '月' +
      d.getDate() +
      '日' +
      d.getHours() +
      '时' +
      d.getMinutes() +
      '分'
    );
  }
}

/**
 * @param {date} time 需要转换的时间
 * @param {String} fmt 需要转换的格式 如 yyyy-MM-dd、yyyy-MM-dd HH:mm:ss
 */
export function formatNewTime(time, fmt) {
  if (!time) return '';
  else {
    const date = new Date(time);
    const o = {
      'M+': date.getMonth() + 1,
      'd+': date.getDate(),
      'H+': date.getHours(),
      'm+': date.getMinutes(),
      's+': date.getSeconds(),
      'q+': Math.floor((date.getMonth() + 3) / 3),
      S: date.getMilliseconds()
    };
    if (/(y+)/.test(fmt)) {
      fmt = fmt.replace(
        RegExp.$1,
        (date.getFullYear() + '').substr(4 - RegExp.$1.length)
      );
    }
    for (const k in o) {
      if (new RegExp('(' + k + ')').test(fmt)) {
        fmt = fmt.replace(
          RegExp.$1,
          RegExp.$1.length === 1
            ? o[k]
            : ('00' + o[k]).substr(('' + o[k]).length)
        );
      }
    }
    return fmt;
  }
}

/**
 * @param {string} url
 * @returns {Object}
 */
export function param2Obj(url) {
  const search = decodeURIComponent(url.split('?')[1]).replace(/\+/g, ' ');
  if (!search) {
    return {};
  }
  const obj = {};
  const searchArr = search.split('&');
  searchArr.forEach(v => {
    const index = v.indexOf('=');
    if (index !== -1) {
      const name = v.substring(0, index);
      const val = v.substring(index + 1, v.length);
      obj[name] = val;
    }
  });
  return obj;
}
/**
  * 字符串拆分数组
  * @param {String} str 拆分参数
  * @param {String} val 拆分条件
  */
export function strToArr(str, val) {
  if (str == 'null') {
    return [];
  }
  return str.split(val);
}
/**
  * 数组转字符串
  * @param {String} str 转换数组
  */
export function arrToStr(arr) {
  return arr.toString();
}
/**
  * 两个数组取并集
  * @param {Array} arr1 数组1
  * @param {Array} arr2 数组2
  */
export function unionArr(arr1, arr2) {
  const arr = arr1.concat(arr2.filter(function (val) { return !(arr1.indexOf(val) > -1); }));
  const data = arr.map(item => {
    return {
      id: item
    };
  });
  return data;
}

/**
  * 两个数组取交集
  * @param {Array} arr1 数组1
  * @param {Array} arr2 数组2
  */
export function intersectionArr(arr1, arr2) {
  const tempArr = arr1.filter(function (val) { return arr2.indexOf(val) > -1; });
  const data = tempArr.map(item => {
    return {
      id: item
    };
  });
  return data;
}
/**
  * 金额转千分位
  * @param {String} money
  */

export function changeMoney(money) {
  if (!money) {
    return 0;
  }
  let a = (money + '').replace(/,/g, '');
  if (a.indexof('.') != -1) {
    if (a.split('.')[1].length > 2) {
      a = Math.round(+a * 100) / 100 + '';
    }
    if (a.split('.').length == 2) {
      return (+a.split('.')[0]).toLocaleString() + '.' + a.split('.')[1];
    } else {
      return (+a.split('.')[0]).toLocaleString();
    }
  } else {
    return (+a.split('.')[0]).toLocaleString();
  }
}

/**
 * @name: 时间格式化
 * @msg: 15 =>  00:00:15
 * @param {String} second 时间
 */
export function transferTime(second) {
  let h = Math.floor(second / 3600);
  let m = Math.floor((second % 3600) / 60);
  let s = Math.floor(second % 60);
  h = h < 10 ? `0${h}` : h;
  m = m < 10 ? `0${m}` : m;
  s = s < 10 ? `0${s}` : s;
  return `${h}:${m}:${s}`;
}

/**
 * 深拷贝
 * @param {Object} source
 * @returns {Object}
 */
export function deepClone(source) {
  if (!source && typeof source !== 'object') {
    throw new Error('error arguments', 'deepClone');
  }
  const targetObj = source.constructor === Array ? [] : {};
  Object.keys(source).forEach(keys => {
    if (source[keys] && typeof source[keys] === 'object') {
      targetObj[keys] = deepClone(source[keys]);
    } else {
      targetObj[keys] = source[keys];
    }
  });
  return targetObj;
}
/**
 * 数组转tree对象
 * @param {Object} source
 * @returns {Object}
 */
export const arrayToTree = (arr, rootId, sort, treeId) => {
  const map = {};
  for (const iterator of arr) {
    map[iterator.sort] = iterator;
  }
  for (const iterator of arr) {
    const key = iterator.treeId;
    if (!(key in map)) continue;
    map[key].childFlowNodeTemplate = iterator;
  }
  return map[rootId];
};
export const arrayToTree2 = (data) => {
  // 数据界限处理
  if (!Array.isArray(data) || !data?.length) return [];
  const map = {};
  const tree = [];
  // 初始化 map，以及 childrenList 属性的添加和初始化
  data.forEach((item) => {
    map[item?.id] = { ...item, childrenList: [] };
  });
  data.forEach((item) => {
    if (!!item?.parentId) {
      // 如果存在父id说明并非根元素；父id对应项的childrenList里面完成当前项的分类
      map?.[item?.parentId].childrenList.push(map[item?.id]);
    } else {
      // 如果不存在父id，也就是根部元素，直接挂在tree里面
      tree.push(map?.[item?.id]);
    }
  });
  return tree;
};
export const treeToArray = (treeObj, rootid) => {
  const temp = []; // 设置临时数组，用来存放队列
  const out = []; // 设置输出数组，用来存放要输出的一维数组
  temp.push(treeObj);
  // 首先把根元素存放入out中
  let pid = rootid;
  const obj = deepClone(treeObj);
  obj.pid = pid;
  delete obj.childFlowNodeTemplate;
  out.push(obj);
  // 对树对象进行广度优先的遍历
  while (temp.length > 0) {
    const first = temp.shift();
    const childFlowNodeTemplate = first.childFlowNodeTemplate;
    if (childFlowNodeTemplate) {
      pid = first.id;
      temp.push(childFlowNodeTemplate);
      const obj = deepClone(childFlowNodeTemplate);
      obj.pid = pid;
      delete obj.childFlowNodeTemplate;
      out.push(obj);
    }
  }
  return out;
};

/**
 * 相关方关联类型
 * @param {String} type 相关方类型
 * @returns {String}
 */
export function filterUserRelationType(type) {
  switch (type) {
    case 'wbs_plan_management_liable':
      return {
        name: '建管负责人',
        desc: ''
      };
    case 'wbs_plan_construction_liable':
      return {
        name: '施工单位负责人',
        desc: '负责更新计划进度，发起报验流程'
      };
    case 'wbs_plan_cost_liable':
      return {
        name: '造价单位负责人',
        desc: ''
      };
    case 'wbs_plan_supervision_liable':
      return {
        name: '监理单位负责人',
        desc: '负责检验批、强条、隐蔽工程的检验'
      };
    case 'wbs_plan_design_liable':
      return {
        name: '设计单位负责人',
        desc: '负责关联图纸、构件'
      };
    default:
      return {
        name: '建管负责人',
        desc: ''
      };
  }
}

/**
 * 质量管理流程状态
 * @param {String} type
 * @returns {String}
 */
export function qualityManageFlowStatus(type) {
  let result = '';
  if (type == 'pending') {
    result = '待办';
  } else if (type == 'done') {
    result = '已办';
  } else if (type == 'waiting_send') {
    result = '待发';
  } else if (type == 'has_been_sent') {
    result = '已发';
  }
  return result;
}
export function approveManageFlowStatus(type) {
  let result = '';
  if (type == 'run') {
    result = '审批中';
  } else if (type == 'withdraw') {
    result = '撤销';
  } else if (type == 'draft') {
    result = '草稿';
  } else if (type == 'termination') {
    result = '终止';
  } else if (type == 'rejected') {
    result = '驳回';
  } else if (type == 'end') {
    result = '完结';
  } else {
    result = '';
  }
  return result;
}

/**
 * //防抖---非立即执行版：
 *触发事件后函数不会立即执行，而是在 n 秒后执行，如果在 n 秒内又触发了事件，则会重新计算函数执行时间

     使用示例（回调函数传匿名非箭头函数）
    changeInput: debounce1(function(val) {
      this.$axios.post(.....)
    }, 500),
* */
export function debounce1(fn, delay) {
  let timer = null;
  return function () {
    const args = arguments;
    const context = this;
    if (timer) {
      clearTimeout(timer);
    }

    timer = setTimeout(function () {
      console.log('111');
      fn.apply(context, args);
    }, delay);
  };
}
// 防抖---立即执行版：
/**
* 立即执行版的意思是触发事件后函数会立即执行，* 然后 n 秒内不触发事件才能继续执行函数的效果。
*   this:让 debounce 函数最终返回的函数 this 指向不变
* arguments : 参数
* */
export function debounce2(func, wait) {
  let timeout;
  return function () {
    const context = this;
    const args = arguments;

    if (timeout) clearTimeout(timeout);

    const callNow = !timeout;
    timeout = setTimeout(() => {
      timeout = null;
    }, wait);
    if (callNow) func.apply(context, args);
  };
}

export function capitalMoney(num) {
  // var num = parseFloat(str);
  var strOutput = '';
  var strUnit = '仟佰拾亿仟佰拾万仟佰拾元角分';
  num += '00';
  var intPos = num.indexOf('.');
  if (intPos >= 0) {
    num = num.substring(0, intPos) + num.substr(intPos + 1, 2);
  }
  strUnit = strUnit.substr(strUnit.length - num.length);
  for (var i = 0; i < num.length; i++) {
    strOutput += '零壹贰叁肆伍陆柒捌玖'.substr(num.substr(i, 1), 1) + strUnit.substr(i, 1);
  }
  return strOutput.replace(/零角零分$/, '整').replace(/零[仟佰拾]/g, '零').replace(/零{2,}/g, '零').replace(/零([亿|万])/g, '$1').replace(/零+元/, '元').replace(/亿零{0,3}万/, '亿').replace(/^元/, '零元');
}

/**
 * /通过文件链接下载
 * @param {String} fileDownloadUrl 文件链接
 * @param {String} fileName 文件名称
 * @returns {File}
 */
export function downloadFileByUrl(fileDownloadUrl, fileName) {
  fetch(fileDownloadUrl).then(res => res.blob()).then(blob => {
    const a = document.createElement('a');
    document.body.appendChild(a);
    a.style.display = 'none';
    // 使用获取到的blob对象创建的url
    const url = window.URL.createObjectURL(blob);
    a.href = url;
    // 指定下载的文件名
    if (fileName) a.download = fileName;
    a.click();
    document.body.removeChild(a);
    // 移除blob对象的url
    window.URL.revokeObjectURL(url);
  });
}

export const importAll = (r) => {
  const cache = {};
  r.keys().forEach(key => cache[key] = r(key));
  return cache;
};

/**
 * /预算管理中树形表格统计汇总方法
 * @param { Object } param 树形表格给的对象参数
 * @returns { Array } sums
 */
export function getSummaries(param) {
  console.log(param);
  const { columns, data } = param;
  const sums = [];
  columns.forEach((column, index) => {
    if (index === 0) {
      sums[index] = '总价';
      return;
    }
    const values = toOneDimensional(data).map(item => Number(item[column.property]));
    console.log(values);
    if (!values.every(value => isNaN(value))) {
      sums[index] = values.reduce((prev, curr) => {
        const value = Number(curr);
        if (!isNaN(value)) {
          return prev + curr;
        } else {
          return prev;
        }
      }, 0);
    }
  });
  return sums;
};
// 为了统计总额对多维树结构递归变成一维
function toOneDimensional(data, arr = []) {
  const tree = deepClone(data);
  tree.forEach(val => {
    if (Object.prototype.hasOwnProperty.call(val, 'children')) {
      arr.push(val);
      toOneDimensional(val.children, arr);
    } else {
      arr.push(val);
    }
  });
  return arr;
};
/**
 * /金额千分位格式化
 * @param {String} number 金额
 */
export function formatMoney(number) {
  return !(number + '').includes('.')
    ? // 就是说1-3位后面一定要匹配3位
    (number + '').replace(/\d{1,3}(?=(\d{3})+$)/g, (match) => {
      return match + ',';
    })
    : (number + '').replace(/\d{1,3}(?=(\d{3})+(\.))/g, (match) => {
      return match + ',';
    });
}
/**
 * 生成一个随机key
  @param {String} number 金额
 */
export function generateRandomId(length) {
  var result = '';
  var characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
  var charactersLength = characters.length;
  for (var i = 0; i < length; i++) {
    result += characters.charAt(Math.floor(Math.random() * charactersLength));
  }
  return result;
}

/**
 * 生成一个UID
  @param {String} number
 */
export function generateUid() {
  var dt = new Date().getTime(); // 当前时间的毫秒数
  var uuid = 'xxxx-xxxx-4xxx-yxxx-xxxxxxxx'.replace(/[xy]/g, function(c) {
    var r = (dt + Math.random()*16)%16 | 0;
    dt = Math.floor(dt/16);
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
  return uuid;
}
/**
 * 地址字符串分隔省市区与具体地址
  @param {String} address
 */
export function addressToArray(address) {
  // 正则表达式匹配省、市、区、街道等信息
  const regex = /^(?<province>[^省]+省|[^自治区]+自治区|[^行政区]+行政区|[^市]+市|[^县]+县)?(?<city>[^市]+市|[^地区]+地区|[^州]+州|[^县]+县)?(?<district>[^区]+区|[^县]+县)?(?<street>[^路]+路)?(?<detail>[^路]+街|[^区]+区)?(?<extra>.+)?$/;
  console.log('regex',address.match(regex))
  const match = address.match(regex).groups;
  // 过滤空值，并返回结果数组
  return Object.values(match).filter(Boolean);
}


/**
 * 查找树结构对象(合并时，相同名称，这个方法是最新)
 */
export function getObjById(list,id,childrenName='childrenList',idName='id') {
  //判断list是否是数组
  if(!list instanceof Array){
    return  null
  }
  //遍历数组
  for(let i in list){
    let item=list[i]
    if(item[idName]===id)
    {
      return item
    }else{
      //查不到继续遍历
      if(item[childrenName]){
        let value= getObjById(item[childrenName],id,childrenName,idName)
      //查询到直接返回
        if(value){
          return value
        }
      }
    }
  }
}
/**
 * 根据id查找所有父级id
 */
 export function findPathById(tree, targetId,childrenName='children',path = []) {
  for (const node of tree) {
    path.push(node.id);
    if (node.id === targetId) {
      return path;
    }
    if (node[childrenName]) {
      const foundPath = findPathById(node[childrenName], targetId,childrenName, [...path]);
      if (foundPath) {
        return foundPath;
      }
    }
    path.pop();
  }
  return null;
}
/**
 * 查看文件跳转
 */
export function viewFile(url) {
  // let viewFileUrl = 'http://192.168.1.220:8012'; // 文件查看地址
  // let viewFileUrl = 'https://reader.runshihua.com';
  // console.log('文件预览',`${viewFileUrl}/onlinePreview?url=${encodeURIComponent(this.$Base64.encode(url))}`)
  let sid = localstorageGet('token') || '';
  window.open(`${viewFileUrl}/onlinePreview?url=${encodeURIComponent(Base64.encode(url))}&sid=${sid}`);
}

/**
 * 检测设备缩放百分比
 */
export function detectZoom() {
  var ratio = 0;
  var screen = window.screen;
  var ua = navigator.userAgent.toLowerCase();
  if (window.devicePixelRatio !== undefined) {
    ratio = window.devicePixelRatio;
  } else if (~ua.indexOf('msie')) {
    if (screen.deviceXDPI && screen.logicalXDPI) {
      ratio = screen.deviceXDPI / screen.logicalXDPI;
    }
  } else if (window.outerWidth !== undefined && window.innerWidth !== undefined) {
    ratio = window.outerWidth / window.innerWidth;
  }
  if (ratio) {
    ratio = Math.round(ratio * 100);
  }
  return ratio; // ratio就是获取到的百分比
}

/* 将YYYYn格式（如20262）转为YYYY年第X季度（如2026年第二季度）
 *@param {string|number} dateCode- 待转换值，如20262/'20262'
 *@returns {string}格式化结果，无效值返回原入参
 */
export function formatToQuarter(dateCode) {
  // 1. 转字符串并去除首尾空格，处理数字入参/带空格的字符串入参
  const str = String(dateCode).trim();
  // 2. 容错校验：必须是5位纯数字，否则返回原值
  if (str.length !== 5 || !/^\d+$/.test(str)) {
    return dateCode;
  }
  // 3. 拆解年份和季度数字：前4位是年，最后1位是季度
  const year = str.slice(0, 4);
  const quarterNum = str.slice(4);
  // 4. 校验季度数：必须是1/2/3/4，否则返回原值
  if (!['1', '2', '3', '4'].includes(quarterNum)) {
    return dateCode;
  }
  // 5. 数字季度转中文季度（建立映射关系，简洁易维护）
  const quarterMap = {
    1: '一',
    2: '二',
    3: '三',
    4: '四'
  };
  // 6. 拼接最终结果
  return `${year}年第${quarterMap[quarterNum]}季度`;
}

/**
 * 计算季度汇总数据（净利润与其他类型共用）
 *
 * 【净利润(net_profit)特殊处理】
 *   - 计划完成额：累加季度内所有月份（Q1=1+2+3月，Q2=4+5+6月...）
 *   - 累计完成额：取当季对应月份的值（Q1取month=1，Q2取month=2...），因为接口返回的actualAmount已是当季累计值
 * 【其他类型】：计划额和完成额都累加季度内所有月份
 *
 * @param {number} targetMonth - 目标月份（1-12的整数），用于确定季度范围
 * @param {object} data - 原始账务数据（包含monthAccountings数组）
 * @param {string} type - 类型标识，用于区分净利润等特殊类型（可选）
 * @param {number} selectedQuarter - 当前选择的季度（1-4），净利润类型时使用（可选）
 * @returns {object} 季度汇总结果：{ plannedAmount: 计划金额, actualAmount: 完成金额 }
 */
export function getQuarterAmountSum(targetMonth, data, type, selectedQuarter) {
  if (!data || !Array.isArray(data.monthAccountings)) {
    return { plannedAmount: 0, actualAmount: 0 };
  }

  let quarterStart, quarterEnd;
  if (targetMonth <= 3) {
    quarterStart = 1;
    quarterEnd = 3;
  } else if (targetMonth <= 6) {
    quarterStart = 4;
    quarterEnd = 6;
  } else if (targetMonth <= 9) {
    quarterStart = 7;
    quarterEnd = 9;
  } else {
    quarterStart = 10;
    quarterEnd = 12;
  }

  let plannedSum = 0;
  let actualSum = 0;
  let firstMonthActual = null;

  // 浮点数加法，避免 0.1 + 0.2 !== 0.3 的精度问题
  const safeAdd = (a, b) => Number((a + b).toFixed(10));

  data.monthAccountings.forEach(item => {
    const month = item.month;
    if (month >= quarterStart && month <= quarterEnd) {
      plannedSum = safeAdd(plannedSum, parseFloat(item.plannedAmount) || 0);
    }
    if (type === 'net_profit' && month === selectedQuarter) {
      firstMonthActual = parseFloat(item.actualAmount) || 0;
    } else if (month >= quarterStart && month <= quarterEnd) {
      actualSum = safeAdd(actualSum, parseFloat(item.actualAmount) || 0);
    }
  });

  return {
    plannedAmount: plannedSum,
    actualAmount: type === 'net_profit' ? firstMonthActual : actualSum
  };
}

