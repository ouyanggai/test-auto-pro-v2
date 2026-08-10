<!-- CommentSection.vue -->
<template>
  <div class="comment-section">
    <!-- 评论列表 -->
    <div v-for="(comment, index) in comments" :key="index" class="comment-card">
      <div class="comment-header">
        <span class="username">{{ comment.userName }}</span>
        <span class="timestamp">{{ comment.time }}</span>
      </div>
      <div class="comment-content">{{ comment.content }}</div>
      <div class="comment-footer">
        <div class="like-container" @click="toggleLike(comment)">
          <svg width="16" height="16" viewBox="0 0 24 24" class="like-icon" :class="{'liked': comment.liked}">
            <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z" />
          </svg>
          <span class="like-count">{{ comment.likeCount }}</span>
        </div>
      </div>
    </div>
    
    <!-- 发表评论区域 -->
    <div class="comment-form">
      <el-input
        type="textarea"
        :rows="3"
        placeholder="写下您的评论..."
        v-model="newComment"
      ></el-input>
      <div class="action-bar">
        <el-button type="primary" @click="submitComment" :disabled="!newComment.trim()">发表评论</el-button>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'CommentSection',
  data() {
    return {
      newComment: '',
      // 初始评论数据（示例）
      comments: [
        {
          id: 1,
          userName: "匿名用户",
          time: "2024-07-31 10:14:36",
          content: "第一条建议明确列出具体实施标准",
          likeCount: 10,
          liked: false
        },
        {
          id: 2,
          userName: "匿名用户",
          time: "2024-07-31 11:25:17",
          content: "界面设计简洁明了，用户体验很好",
          likeCount: 5,
          liked: false
        }
      ]
    }
  },
  methods: {
    // 点赞/取消点赞
    toggleLike(comment) {
      if (comment.liked) {
        comment.likeCount--;
      } else {
        comment.likeCount++;
      }
      comment.liked = !comment.liked;
    },
    
    // 提交新评论
    submitComment() {
      if (!this.newComment.trim()) return;
      
      const newComment = {
        id: Date.now(),
        userName: "匿名用户",
        time: this.getCurrentTime(),
        content: this.newComment,
        likeCount: 0,
        liked: false
      };
      
      this.comments.unshift(newComment);
      this.newComment = "";
      
      // 模拟提交成功提示
      this.$message.success("评论发表成功");
    },
    
    // 获取当前时间
    getCurrentTime() {
      const now = new Date();
      return `${now.getFullYear()}-${(now.getMonth()+1).toString().padStart(2,'0')}-${now.getDate().toString().padStart(2,'0')} ${now.getHours().toString().padStart(2,'0')}:${now.getMinutes().toString().padStart(2,'0')}:${now.getSeconds().toString().padStart(2,'0')}`;
    }
  }
}
</script>

<style scoped>
.comment-section {
  max-width: 650px;
  margin: 0 auto;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
  background-color: #f5f7fa;
  padding: 20px;
  border-radius: 8px;
}

.comment-card {
  background-color: white;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.05), 0 1px 2px rgba(0,0,0,0.1);
  padding: 16px;
  margin-bottom: 20px;
  position: relative;
}

.comment-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.username {
  color: #1890ff;
  font-weight: 500;
  font-size: 14px;
}

.timestamp {
  color: #8c8c8c;
  font-size: 12px;
}

.comment-content {
  color: #595959;
  font-size: 14px;
  line-height: 1.5;
  margin-bottom: 16px;
  padding-left: 2px;
}

.comment-footer {
  display: flex;
  justify-content: flex-end;
}

.like-container {
  display: flex;
  align-items: center;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.like-container:hover {
  background-color: #f5f5f5;
}

.like-icon {
  width: 16px;
  height: 16px;
  fill: #bfbfbf;
  transition: all 0.3s ease;
}

.like-icon:hover {
  fill: #f759ab;
}

.like-icon.liked {
  fill: #f5222d;
}

.like-count {
  color: #8c8c8c;
  font-size: 12px;
  margin-left: 4px;
}

.comment-form {
  background-color: white;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.05), 0 1px 2px rgba(0,0,0,0.1);
  padding: 16px;
  margin-top: 24px;
}

.action-bar {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

/* Element UI 输入框调整 */
/deep/ .el-textarea__inner {
  border: none;
  border-bottom: 1px solid #e8e8e8;
  border-radius: 0;
  padding: 8px 2px;
  resize: none;
}

/deep/ .el-textarea__inner:focus {
  border-bottom-color: #1890ff;
  box-shadow: none;
}

/deep/ .el-textarea__inner::placeholder {
  color: #bfbfbf;
}

/* 按钮样式调整 */
/deep/ .el-button {
  padding: 8px 16px;
  font-size: 14px;
  height: auto;
}

/deep/ .el-button--primary {
  background-color: #1890ff;
  border-color: #1890ff;
}

/deep/ .el-button--primary:hover {
  background-color: #40a9ff;
  border-color: #40a9ff;
}

/deep/ .el-button.is-disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>