export function useContractRateFeature() {
  const config = useRuntimeConfig()
  const enabled = computed(() => config.public.contractRateWorkspaceEnabled === true)
  return { enabled }
}
