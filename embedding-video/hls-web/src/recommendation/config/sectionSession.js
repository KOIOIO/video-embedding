export const RECOMMENDATION_SECTION_STORAGE_KEY = 'hstv-recommendation-console.active-section'

function resolveStorage(storage) {
  return storage === undefined ? globalThis.localStorage : storage
}

export function isKnownSection(section, navigationItems) {
  return Boolean(section && navigationItems.some((item) => item.key === section))
}

export function readActiveSection(fallbackSection, navigationItems, storage) {
  try {
    const storedSection = resolveStorage(storage)?.getItem(RECOMMENDATION_SECTION_STORAGE_KEY)
    return isKnownSection(storedSection, navigationItems) ? storedSection : fallbackSection
  } catch {
    return fallbackSection
  }
}

export function writeActiveSection(section, navigationItems, storage) {
  if (!isKnownSection(section, navigationItems)) {
    return false
  }

  try {
    const resolvedStorage = resolveStorage(storage)
    if (!resolvedStorage) return false
    resolvedStorage.setItem(RECOMMENDATION_SECTION_STORAGE_KEY, section)
    return true
  } catch {
    return false
  }
}
