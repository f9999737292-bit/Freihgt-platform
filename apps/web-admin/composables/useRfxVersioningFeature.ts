/** Feature gate for RFx v3.0E template library and versioning UI. */

export function useRfxVersioningFeature() {
  const config = useRuntimeConfig()

  const enabled = computed(
    () => config.public.rfxVersioningV3Enabled === true
      || String(config.public.rfxVersioningV3Enabled) === 'true',
  )

  function requireEnabled(): boolean {
    return enabled.value
  }

  return { enabled, requireEnabled }
}
