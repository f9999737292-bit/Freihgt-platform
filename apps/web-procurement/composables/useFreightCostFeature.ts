export function useFreightCostFeature() {
  const config = useRuntimeConfig()
  const enabled = computed(() => config.public.freightCostWorkspaceEnabled === true)
  return { enabled }
}
