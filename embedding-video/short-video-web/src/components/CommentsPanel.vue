<script setup>
import { computed, onMounted, ref } from 'vue'
import {
  COMMENT_REACTION_DISLIKE,
  COMMENT_REACTION_DOUBLE_LIKE,
  COMMENT_REACTION_LIKE,
  createComment,
  createReply,
  fetchComments,
  fetchReplies,
  toggleCommentReaction,
} from '../feed/commentApi.js'
import { formatRelativeTime } from '../feed/relativeTime.js'
import { searchUsers } from '../user/api.js'

const props = defineProps({
  segmentId: { type: Number, required: true },
  userId: { type: Number, default: 0 },
})
const emit = defineEmits(['close', 'total-change'])

const PAGE_SIZE = 10

const comments = ref([])
const total = ref(0)
const loading = ref(true)
const loadError = ref('')
const page = ref(0)
const sending = ref(false)
const inputText = ref('')
const replyTarget = ref(null)
const expandedReplies = ref({})
const reactionBusy = ref(false)

// --- @提及用户状态 ---
const mentionOpen = ref(false)
const mentionKeyword = ref('')
const mentionResults = ref([])
const mentionLoading = ref(false)
const mentionActiveIndex = ref(0)
const mentionStartPos = ref(-1)
let searchTimer = null

const hasMore = computed(() => comments.value.length < total.value)
const inputPlaceholder = computed(() => {
  if (replyTarget.value) return `回复 @${replyTarget.value.username}`
  return '有爱评论，说点儿什么…'
})

async function load(reset = false) {
  loading.value = true
  loadError.value = ''
  try {
    const nextPage = reset ? 1 : page.value + 1
    const list = await fetchComments({
      segmentId: props.segmentId,
      userId: props.userId,
      page: nextPage,
      pageSize: PAGE_SIZE,
    })
    if (reset) {
      comments.value = list.comments
    } else {
      comments.value = comments.value.concat(list.comments)
    }
    total.value = list.total
    page.value = nextPage
  } catch {
    loadError.value = '评论加载失败'
  } finally {
    loading.value = false
  }
}

function onScroll(event) {
  const el = event.target
  if (loading.value || !hasMore.value) return
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 80) {
    void load(false)
  }
}

function applyReactionChange(comment, reactionType) {
  const previous = comment.user_reaction_type
  if (previous === reactionType) {
    comment.user_reaction_type = ''
    if (reactionType === COMMENT_REACTION_LIKE) comment.like_count = Math.max(0, comment.like_count - 1)
    if (reactionType === COMMENT_REACTION_DOUBLE_LIKE) comment.double_like_count = Math.max(0, comment.double_like_count - 1)
    return
  }
  if (previous === COMMENT_REACTION_LIKE) comment.like_count = Math.max(0, comment.like_count - 1)
  if (previous === COMMENT_REACTION_DOUBLE_LIKE) comment.double_like_count = Math.max(0, comment.double_like_count - 1)
  comment.user_reaction_type = reactionType
  if (reactionType === COMMENT_REACTION_LIKE) comment.like_count += 1
  if (reactionType === COMMENT_REACTION_DOUBLE_LIKE) comment.double_like_count += 1
}

async function onToggleReaction(comment, reactionType) {
  if (reactionBusy.value || !comment.id) return
  const previous = {
    type: comment.user_reaction_type,
    like: comment.like_count,
    doubleLike: comment.double_like_count,
  }
  applyReactionChange(comment, reactionType)
  reactionBusy.value = true
  try {
    const result = await toggleCommentReaction({ commentId: comment.id, reactionType })
    comment.user_reaction_type = result.active ? result.reaction_type : ''
    comment.like_count = result.like_count
    comment.double_like_count = result.double_like_count
  } catch {
    comment.user_reaction_type = previous.type
    comment.like_count = previous.like
    comment.double_like_count = previous.doubleLike
  } finally {
    reactionBusy.value = false
  }
}

function openReply(comment, rootComment = null) {
  replyTarget.value = {
    id: comment.id,
    username: comment.username,
    rootId: (rootComment || comment).id,
  }
}

function cancelReply() {
  replyTarget.value = null
}

async function expandReplies(comment) {
  if (expandedReplies.value[comment.id]) return
  const all = []
  let pageNo = 1
  for (;;) {
    const list = await fetchReplies({ commentId: comment.id, userId: props.userId, page: pageNo, pageSize: 20 })
    all.push(...list.comments)
    if (all.length >= list.total || list.comments.length === 0) break
    pageNo += 1
  }
  expandedReplies.value = { ...expandedReplies.value, [comment.id]: all }
}

function repliesOf(comment) {
  if (expandedReplies.value[comment.id]) return expandedReplies.value[comment.id]
  return comment.replies
}

async function submit() {
  const content = inputText.value.trim()
  if (!content || sending.value) return
  sending.value = true
  try {
    if (replyTarget.value) {
      const reply = await createReply({ commentId: replyTarget.value.id, content })
      const root = comments.value.find((c) => c.id === replyTarget.value.rootId)
      if (root) {
        root.replies = root.replies.concat([reply])
        root.reply_count += 1
        if (expandedReplies.value[root.id]) {
          expandedReplies.value = {
            ...expandedReplies.value,
            [root.id]: expandedReplies.value[root.id].concat([reply]),
          }
        }
      }
      replyTarget.value = null
    } else {
      const comment = await createComment({ segmentId: props.segmentId, content })
      comments.value = [comment].concat(comments.value)
    }
    total.value += 1
    emit('total-change', total.value)
    inputText.value = ''
  } catch {
    inputText.value = content
  } finally {
    sending.value = false
  }
}

// --- @提及用户功能 ---

function onInput(event) {
  const input = event.target
  const cursor = input.selectionStart
  const text = inputText.value

  // 从光标位置向前查找最近的 "@"
  let atPos = -1
  for (let i = cursor - 1; i >= 0; i--) {
    if (text[i] === '@') {
      atPos = i
      break
    }
    // @ 后不能有空格，如果遇到空格说明不是在输入提及
    if (text[i] === ' ') break
  }

  if (atPos === -1) {
    closeMention()
    return
  }

  // 提取 @ 后的关键词（到光标位置，不含空格）
  const keyword = text.slice(atPos + 1, cursor)
  if (keyword.includes(' ')) {
    closeMention()
    return
  }

  mentionStartPos.value = atPos
  mentionKeyword.value = keyword
  mentionOpen.value = true

  // 防抖 300ms 搜索
  if (searchTimer) clearTimeout(searchTimer)
  if (keyword.length === 0) {
    mentionResults.value = []
    mentionLoading.value = false
    return
  }
  mentionLoading.value = true
  searchTimer = setTimeout(async () => {
    try {
      const result = await searchUsers(keyword, 1, 20)
      mentionResults.value = result.list
      mentionActiveIndex.value = 0
    } catch {
      mentionResults.value = []
    } finally {
      mentionLoading.value = false
    }
  }, 300)
}

function selectMention(user) {
  if (!user) return
  const input = document.querySelector('.comment-input')
  const cursor = mentionStartPos.value + mentionKeyword.value.length + 1
  const before = inputText.value.slice(0, mentionStartPos.value)
  const after = inputText.value.slice(cursor)
  const replacement = `@${user.nickname} `
  inputText.value = before + replacement + after

  // 设置光标到替换后的末尾
  const newCursor = before.length + replacement.length
  if (input) {
    requestAnimationFrame(() => {
      input.focus()
      input.setSelectionRange(newCursor, newCursor)
    })
  }
  closeMention()
}

function closeMention() {
  mentionOpen.value = false
  mentionKeyword.value = ''
  mentionResults.value = []
  mentionLoading.value = false
  mentionActiveIndex.value = 0
  mentionStartPos.value = -1
  if (searchTimer) {
    clearTimeout(searchTimer)
    searchTimer = null
  }
}

function onMentionKeydown(event) {
  if (!mentionOpen.value) return
  const visible = mentionResults.value.slice(0, 8)

  if (event.key === 'ArrowDown') {
    event.preventDefault()
    if (visible.length > 0) {
      mentionActiveIndex.value = (mentionActiveIndex.value + 1) % visible.length
    }
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    if (visible.length > 0) {
      mentionActiveIndex.value = (mentionActiveIndex.value - 1 + visible.length) % visible.length
    }
  } else if (event.key === 'Enter') {
    if (visible.length > 0 && mentionActiveIndex.value < visible.length) {
      event.preventDefault()
      selectMention(visible[mentionActiveIndex.value])
    }
  } else if (event.key === 'Escape') {
    event.preventDefault()
    closeMention()
  }
}

onMounted(() => load(true))
</script>

<template>
  <div class="panel-wrap" @click.self="emit('close')">
    <div class="panel">
      <header class="panel-head">
        <span class="panel-title">{{ total }} 条评论</span>
        <button class="close-btn" type="button" aria-label="关闭" @click="emit('close')">
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M6.2 5.1a.8.8 0 0 0-1.1 1.1L10.9 12l-5.8 5.8a.8.8 0 1 0 1.1 1.1L12 13.1l5.8 5.8a.8.8 0 1 0 1.1-1.1L13.1 12l5.8-5.8a.8.8 0 0 0-1.1-1.1L12 10.9 6.2 5.1Z"/></svg>
        </button>
      </header>

      <div class="panel-body" @scroll.passive="onScroll">
        <div v-if="loading" class="state">评论加载中…</div>
        <div v-else-if="loadError" class="state">
          <p>{{ loadError }}</p>
          <button class="retry-btn" type="button" @click="load(true)">重试</button>
        </div>
        <div v-else-if="comments.length === 0" class="state">还没有评论，来抢沙发～</div>

        <template v-else>
          <div v-for="comment in comments" :key="comment.id" class="comment">
            <div class="avatar">{{ comment.username.slice(0, 1) || '?' }}</div>
            <div class="comment-main">
              <div class="comment-meta">
                <span class="comment-username">{{ comment.username || `用户${comment.user_id}` }}</span>
                <span class="comment-time">{{ formatRelativeTime(comment.created_at_unix) }}</span>
              </div>
              <p class="comment-content">{{ comment.content }}</p>
              <div class="comment-actions">
                <button
                  class="reaction-btn"
                  :class="{ active: comment.user_reaction_type === COMMENT_REACTION_LIKE }"
                  type="button"
                  @click="onToggleReaction(comment, COMMENT_REACTION_LIKE)"
                >
                  <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/></svg>
                  <span v-if="comment.like_count > 0">{{ comment.like_count }}</span>
                </button>
                <button
                  class="reaction-btn"
                  :class="{ active: comment.user_reaction_type === COMMENT_REACTION_DOUBLE_LIKE }"
                  type="button"
                  @click="onToggleReaction(comment, COMMENT_REACTION_DOUBLE_LIKE)"
                >
                  <svg viewBox="0 0 24 24" fill="currentColor">
                    <path transform="translate(1.8 1.2) scale(0.7)" d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/>
                    <path transform="translate(-1.8 0.4) scale(0.7)" d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/>
                  </svg>
                  <span v-if="comment.double_like_count > 0">{{ comment.double_like_count }}</span>
                </button>
                <button
                  class="reaction-btn"
                  :class="{ active: comment.user_reaction_type === COMMENT_REACTION_DISLIKE }"
                  type="button"
                  @click="onToggleReaction(comment, COMMENT_REACTION_DISLIKE)"
                >
                  <svg viewBox="0 0 24 24" fill="currentColor">
                    <path transform="rotate(180 12 12)" d="M7 10v11a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V10a1 1 0 0 1 1-1h1a1 1 0 0 1 1 1Zm13.8.8c-.3-1.2-1.4-2-2.7-2h-3.7a.3.3 0 0 1-.3-.4l.7-3.6a3 3 0 0 0-2.9-3.4 2 2 0 0 0-1.8 1.2l-2.5 5.2a3 3 0 0 0-.3 1.2v6.5a3 3 0 0 0 3 3h6.4a3 3 0 0 0 2.9-2.4l1.2-4.3v-1Z"/>
                  </svg>
                </button>
                <button class="reply-btn" type="button" @click="openReply(comment, comment)">回复</button>
              </div>

              <div v-if="comment.replies.length > 0 || comment.reply_count > 0" class="replies">
                <div v-for="reply in repliesOf(comment)" :key="reply.id" class="reply">
                  <span class="reply-name">{{ reply.username || `用户${reply.user_id}` }}</span>
                  <span v-if="reply.reply_to_username" class="reply-to">回复 {{ reply.reply_to_username }}</span>
                  <span class="reply-colon">：</span>
                  <span class="reply-content">{{ reply.content }}</span>
                  <span class="reply-time">{{ formatRelativeTime(reply.created_at_unix) }}</span>
                  <span class="reply-reactions">
                    <button
                      class="reply-reaction"
                      :class="{ active: reply.user_reaction_type === COMMENT_REACTION_LIKE }"
                      type="button"
                      @click="onToggleReaction(reply, COMMENT_REACTION_LIKE)"
                    >
                      <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/></svg>
                      <span v-if="reply.like_count > 0">{{ reply.like_count }}</span>
                    </button>
                    <button
                      class="reply-reaction"
                      :class="{ active: reply.user_reaction_type === COMMENT_REACTION_DOUBLE_LIKE }"
                      type="button"
                      @click="onToggleReaction(reply, COMMENT_REACTION_DOUBLE_LIKE)"
                    >
                      <svg viewBox="0 0 24 24" fill="currentColor">
                        <path transform="translate(1.8 1.2) scale(0.7)" d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/>
                        <path transform="translate(-1.8 0.4) scale(0.7)" d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/>
                      </svg>
                      <span v-if="reply.double_like_count > 0">{{ reply.double_like_count }}</span>
                    </button>
                    <button
                      class="reply-reaction"
                      :class="{ active: reply.user_reaction_type === COMMENT_REACTION_DISLIKE }"
                      type="button"
                      @click="onToggleReaction(reply, COMMENT_REACTION_DISLIKE)"
                    >
                      <svg viewBox="0 0 24 24" fill="currentColor">
                        <path transform="rotate(180 12 12)" d="M7 10v11a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V10a1 1 0 0 1 1-1h1a1 1 0 0 1 1 1Zm13.8.8c-.3-1.2-1.4-2-2.7-2h-3.7a.3.3 0 0 1-.3-.4l.7-3.6a3 3 0 0 0-2.9-3.4 2 2 0 0 0-1.8 1.2l-2.5 5.2a3 3 0 0 0-.3 1.2v6.5a3 3 0 0 0 3 3h6.4a3 3 0 0 0 2.9-2.4l1.2-4.3v-1Z"/>
                      </svg>
                    </button>
                  </span>
                  <button class="reply-reply" type="button" @click="openReply(reply, comment)">回复</button>
                </div>

                <button
                  v-if="comment.reply_count > comment.replies.length && !expandedReplies[comment.id]"
                  class="expand-replies"
                  type="button"
                  @click="expandReplies(comment)"
                >
                  展开全部 {{ comment.reply_count }} 条回复
                </button>
              </div>
            </div>
          </div>

          <div v-if="hasMore" class="state more-hint">上滑加载更多…</div>
          <div v-else class="state more-hint">— 到底啦 —</div>
        </template>
      </div>

      <div class="input-bar">
        <button v-if="replyTarget" class="cancel-reply" type="button" @click="cancelReply">✕</button>
        <div class="input-wrap">
          <div v-if="mentionOpen" class="mention-dropdown">
            <div v-if="mentionLoading" class="mention-loading">搜索中…</div>
            <div v-else-if="mentionResults.length === 0" class="mention-empty">无匹配用户</div>
            <div
              v-for="(user, idx) in mentionResults.slice(0, 8)"
              :key="user.id"
              class="mention-item"
              :class="{ active: idx === mentionActiveIndex }"
              @click="selectMention(user)"
            >
              <div class="mention-avatar">
                <img v-if="user.avatar_url" :src="user.avatar_url" alt="" />
                <span v-else>{{ user.nickname?.charAt(0) || '?' }}</span>
              </div>
              <div class="mention-info">
                <span class="mention-nickname">{{ user.nickname }}</span>
                <span v-if="user.is_following" class="mention-following">已关注</span>
              </div>
            </div>
          </div>
          <input
            v-model="inputText"
            class="comment-input"
            type="text"
            :placeholder="inputPlaceholder"
            maxlength="500"
            @input="onInput"
            @keydown="onMentionKeydown"
            @keyup.enter="!mentionOpen && submit()"
          />
        </div>
        <button class="send-btn" type="button" :disabled="!inputText.trim() || sending" @click="submit">
          发送
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.panel-wrap {
  position: fixed;
  inset: 0;
  z-index: 30;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  background: rgba(0, 0, 0, 0.45);
  animation: fade-in 0.2s ease both;
}

.panel {
  display: flex;
  flex-direction: column;
  height: 74%;
  border-radius: 16px 16px 0 0;
  background: #151515;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  overflow: hidden;
  animation: sheet-up 0.25s cubic-bezier(0.25, 0.8, 0.25, 1) both;
}

@keyframes sheet-up {
  from {
    transform: translateY(40px);
    opacity: 0.6;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}

.panel-head {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.panel-title {
  font-size: 15px;
  font-weight: 600;
  color: #fff;
}

.close-btn {
  position: absolute;
  right: 14px;
  top: 50%;
  transform: translateY(-50%);
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  color: rgba(255, 255, 255, 0.7);
}

.close-btn svg {
  width: 18px;
  height: 18px;
}

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 6px 16px 12px;
  overscroll-behavior: contain;
}

.state {
  padding: 40px 0;
  text-align: center;
  color: var(--text-faint);
  font-size: 13px;
}

.retry-btn {
  margin-top: 10px;
  padding: 8px 22px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
  font-size: 13px;
}

.more-hint {
  padding: 16px 0 6px;
}

.comment {
  display: flex;
  gap: 10px;
  padding: 12px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.avatar {
  flex-shrink: 0;
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: linear-gradient(135deg, #25f4ee, #3a86ff);
  font-size: 15px;
  font-weight: 700;
  color: #04252b;
}

.comment-main {
  flex: 1;
  min-width: 0;
}

.comment-meta {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.comment-username {
  font-size: 12px;
  color: var(--text-faint);
}

.comment-time {
  font-size: 11px;
  color: var(--text-faint);
}

.comment-content {
  margin: 4px 0 6px;
  font-size: 14px;
  line-height: 1.5;
  color: rgba(255, 255, 255, 0.94);
  white-space: pre-wrap;
  word-break: break-word;
}

.comment-actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.reaction-btn,
.reply-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--text-faint);
}

.reaction-btn svg {
  width: 15px;
  height: 15px;
}

.reaction-btn.active {
  color: var(--accent);
}

.replies {
  margin-top: 8px;
  padding: 8px 10px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.045);
}

.reply {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 4px;
  padding: 4px 0;
  font-size: 13px;
  line-height: 1.5;
}

.reply-name {
  color: #7fb8c9;
  font-weight: 600;
}

.reply-to {
  color: #7fb8c9;
}

.reply-colon {
  color: rgba(255, 255, 255, 0.35);
}

.reply-content {
  flex: 1;
  min-width: 60%;
  color: rgba(255, 255, 255, 0.92);
  word-break: break-word;
}

.reply-time {
  font-size: 11px;
  color: var(--text-faint);
}

.reply-reactions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.reply-reaction {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  color: var(--text-faint);
}

.reply-reaction svg {
  width: 13px;
  height: 13px;
}

.reply-reaction.active {
  color: var(--accent);
}

.reply-reply {
  font-size: 11px;
  color: var(--text-faint);
}

.expand-replies {
  display: block;
  margin-top: 4px;
  font-size: 12px;
  color: #7fb8c9;
}

.input-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px calc(10px + env(safe-area-inset-bottom));
  background: #1c1c1c;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.cancel-reply {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.7);
  font-size: 12px;
}

.comment-input {
  flex: 1;
  padding: 10px 14px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.07);
  color: #fff;
  font-size: 14px;
  outline: none;
}

.comment-input::placeholder {
  color: var(--text-faint);
}

.send-btn {
  flex-shrink: 0;
  padding: 9px 18px;
  border-radius: 999px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 13px;
  font-weight: 700;
}

.send-btn:disabled {
  opacity: 0.45;
  cursor: default;
}

/* --- @提及用户下拉 --- */

.input-wrap {
  position: relative;
  flex: 1;
}

.mention-dropdown {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 0;
  right: 0;
  max-height: 280px;
  overflow-y: auto;
  background: #1c1c1e;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  box-shadow: 0 -4px 24px rgba(0, 0, 0, 0.5);
  z-index: 10;
  animation: dropdown-up 0.15s ease both;
}

@keyframes dropdown-up {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.mention-loading,
.mention-empty {
  padding: 14px 16px;
  font-size: 13px;
  color: var(--text-faint);
  text-align: center;
}

.mention-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  cursor: pointer;
  transition: background 0.12s;
}

.mention-item:hover,
.mention-item.active {
  background: rgba(255, 255, 255, 0.08);
}

.mention-avatar {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: linear-gradient(135deg, #25f4ee, #3a86ff);
  font-size: 13px;
  font-weight: 700;
  color: #04252b;
  overflow: hidden;
}

.mention-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.mention-info {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.mention-nickname {
  font-size: 14px;
  color: rgba(255, 255, 255, 0.94);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mention-following {
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(254, 44, 85, 0.15);
  color: #fe2c55;
  font-size: 11px;
  font-weight: 600;
}
</style>
