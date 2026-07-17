export function normalizePerformancePoints(points) {
  if (!Array.isArray(points)) {
    return []
  }
  return points
    .map((point) => {
      const time = Date.parse(point?.timestamp)
      const value = Number(point?.value)
      if (!Number.isFinite(time) || !Number.isFinite(value)) {
        return null
      }
      return {
        timestamp: new Date(time).toISOString(),
        time,
        value,
      }
    })
    .filter(Boolean)
    .sort((a, b) => a.time - b.time)
}

export function buildTrendGeometry(rawPoints, options = {}) {
  const width = positiveNumber(options.width, 720)
  const height = positiveNumber(options.height, 280)
  const padding = Math.min(positiveNumber(options.padding, 36), width / 3, height / 3)
  const points = normalizePerformancePoints(rawPoints)
  if (!points.length) {
    return emptyGeometry(width, height, padding)
  }

  const plotWidth = width - padding * 2
  const plotHeight = height - padding * 2
  const firstTime = points[0].time
  const lastTime = points[points.length - 1].time
  const values = points.map((point) => point.value)
  let min = Math.min(...values)
  let max = Math.max(...values)
  if (min === max) {
    const delta = Math.abs(min) * 0.1 || 1
    min -= delta
    max += delta
  } else {
    const delta = (max - min) * 0.08
    min -= delta
    max += delta
  }

  const plotted = points.map((point) => ({
    ...point,
    x: roundCoordinate(firstTime === lastTime
      ? width / 2
      : padding + ((point.time - firstTime) / (lastTime - firstTime)) * plotWidth),
    y: roundCoordinate(padding + ((max - point.value) / (max - min)) * plotHeight),
  }))
  const linePath = plotted
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`)
    .join(' ')
  const baseline = height - padding
  const areaPath = `M ${plotted[0].x} ${baseline} ${plotted.map((point) => `L ${point.x} ${point.y}`).join(' ')} L ${plotted[plotted.length - 1].x} ${baseline} Z`

  return {
    width,
    height,
    padding,
    min,
    max,
    points: plotted,
    linePath,
    areaPath,
    xTicks: selectTicks(plotted, 5),
    yTicks: buildYTicks(min, max, padding, plotHeight),
  }
}

export function formatPerformanceValue(value) {
  const number = Number(value)
  if (!Number.isFinite(number)) {
    return '-'
  }
  return String(Number(number.toFixed(5)))
}

function emptyGeometry(width, height, padding) {
  return {
    width,
    height,
    padding,
    min: 0,
    max: 0,
    points: [],
    linePath: '',
    areaPath: '',
    xTicks: [],
    yTicks: [],
  }
}

function selectTicks(points, limit) {
  if (points.length <= limit) {
    return points
  }
  const indexes = new Set([0, points.length - 1])
  for (let i = 1; i < limit - 1; i += 1) {
    indexes.add(Math.round((i * (points.length - 1)) / (limit - 1)))
  }
  return [...indexes].sort((a, b) => a - b).map((index) => points[index])
}

function buildYTicks(min, max, padding, plotHeight) {
  return Array.from({ length: 5 }, (_, index) => {
    const ratio = index / 4
    return {
      value: max - (max - min) * ratio,
      y: roundCoordinate(padding + plotHeight * ratio),
    }
  })
}

function positiveNumber(value, fallback) {
  const number = Number(value)
  return Number.isFinite(number) && number > 0 ? number : fallback
}

function roundCoordinate(value) {
  return Number(value.toFixed(2))
}
