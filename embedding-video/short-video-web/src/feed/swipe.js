export function clampIndex(index, maxIndex) {
  if (maxIndex <= 0) return 0
  return Math.min(maxIndex, Math.max(0, index))
}

export function resolveSwipeIndex(deltaY, currentIndex, maxIndex, threshold = 60) {
  if (Math.abs(deltaY) < threshold) return currentIndex
  return clampIndex(currentIndex + (deltaY > 0 ? 1 : -1), maxIndex)
}

export function visibleWindow(index, count) {
  if (count <= 0) return [0, -1]
  const start = Math.max(0, index - 1)
  const end = Math.min(count - 1, index + 1)
  return [start, end]
}

export function pushDistinct(seenKeys, key, cap = 30) {
  if (seenKeys.has(key)) return
  seenKeys.add(key)
  while (seenKeys.size > cap) {
    seenKeys.delete(seenKeys.values().next().value)
  }
}
