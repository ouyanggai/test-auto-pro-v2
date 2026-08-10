/*
 * @Descripttion:并发控制类
const uploadQueue = new RequestQueue(20);
Array.from({ length: 100000 }).forEach(() => {
  uploadQueue.enqueue(() => 
    axios.post(url, data, {
      timeout: 30000,
      ...config
    })
  ).then(response => {
    console.log('上传成功', response);
  }).catch(error => {
    console.error('上传失败', error);
  });
})
 */

class RequestQueue {
  constructor(maxConcurrent = 20) {
    this.maxConcurrent = maxConcurrent; // 最大并发数
    this.activeCount = 0; // 当前活跃请求数
    this.queue = []; // 等待队列
    this.allCompletedCallbacks = []; // 存储所有请求完成后的回调函数
  }

  // 注册"全部请求完成"回调
  onAllCompleted(callback) {
    if (typeof callback === 'function') {
      this.allCompletedCallbacks.push(callback);
    }
  }

  // 触发所有完成回调
  _triggerAllCompleted() {
    if (this.activeCount === 0 && this.queue.length === 0) {
      this.allCompletedCallbacks.forEach(cb => cb());
    }
  }

  // 添加请求到队列
  enqueue(requestFn) {
    return new Promise((resolve, reject) => {
      this.queue.push({
        requestFn,
        resolve,
        reject
      });
      this.tryNext();
    });
  }

  // 尝试执行下一个请求
  tryNext() {
    if (this.activeCount >= this.maxConcurrent || !this.queue.length) return;

    const {
      requestFn,
      resolve,
      reject
    } = this.queue.shift();
    this.activeCount++;

    requestFn()
      .then((response) => {
        resolve(response);
      })
      .catch((error) => {
        reject(error);
      })
      .finally(() => {
        this.activeCount--;
        this.tryNext(); // 无论成功/失败都继续执行下一个
        this._triggerAllCompleted(); // 每次请求结束后检查是否全部完成
      });
  }
}

export default RequestQueue;
