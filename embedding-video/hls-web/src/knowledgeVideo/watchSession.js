export function createWatchSession({
  knowledgeVideoId,
  userId,
  sessionId = crypto.randomUUID(),
  report,
}) {
  let acknowledged = 0
  let queue = Promise.resolve()

  return {
    id: sessionId,
    acknowledgedSeconds: () => acknowledged,
    flush(watchedSeconds, { keepalive = false } = {}) {
      const seconds = Math.max(0, Math.floor(Number(watchedSeconds) || 0))
      const operation = queue.catch(() => {}).then(async () => {
        if (seconds <= acknowledged) return null
        const result = await report({
          knowledgeVideoId,
          sessionId,
          userId,
          watchedSeconds: seconds,
          keepalive,
        })
        acknowledged = Math.max(acknowledged, seconds)
        return result
      })
      queue = operation
      return operation
    },
  }
}
