<script setup lang="ts">
import { Button, CustomerAppShell, CustomerNavigation, LoadingState, LocaleSwitcher, Select } from '@freight-platform/ui'
import CompanyGate from '../components/CompanyGate.vue'
import PortalState from '../components/PortalState.vue'

const office = useShipperOffice()
const route = useRoute()
const { t } = useI18n()
const localePath = useLocalePath()

const items = computed(() => [
  { href: localePath('/'), label: t('common.home'), active: route.path === localePath('/') },
  { href: localePath('/shipments'), label: t('shipper.navShipments'), active: route.path.startsWith(localePath('/shipments')) },
  { href: localePath('/documents'), label: t('shipper.navDocuments'), active: route.path.startsWith(localePath('/documents')) },
])

const companyOptions = computed(() => office.companies.value.map((item) => ({
  value: item.companyId,
  label: item.legalName || item.companyId,
})))

const companyModel = computed({
  get: () => office.selectedCompanyId.value ?? '',
  set: (value: string) => {
    if (value && value !== office.selectedCompanyId.value) office.chooseCompany(value)
  },
})

function onSelect(companyId: string) {
  office.chooseCompany(companyId)
}

onMounted(() => {
  void office.refreshMemberships()
})
</script>

<template>
  <ClientOnly>
    <CustomerAppShell :title="t('shipper.product')">
      <template #nav>
        <CustomerNavigation :items="items" />
      </template>
      <template #context>
        <span v-if="office.session.value">{{ office.session.value.user.email }}</span>
        <Select
          v-if="companyOptions.length > 1 && office.selectedCompanyId.value"
          v-model="companyModel"
          name="company"
          data-testid="company-switch"
          :label="t('shipper.companyLabel')"
          :options="companyOptions"
        />
      </template>
      <template #actions>
        <LocaleSwitcher />
        <Button variant="ghost" type="button" data-testid="sign-out" @click="office.signOut()">
          {{ t('shipper.signOut') }}
        </Button>
      </template>
      <PortalState
        v-if="office.membershipError.value"
        kind="unavailable"
        :title="t('shipper.unavailable')"
      />
      <LoadingState v-else-if="!office.membershipReady.value" :label="t('shipper.loading')" />
      <CompanyGate
        v-else-if="!office.selectedCompanyId.value"
        :companies="office.companies.value"
        @select="onSelect"
      />
      <slot v-else />
    </CustomerAppShell>
    <template #fallback>
      <p>{{ t('shipper.loading') }}</p>
    </template>
  </ClientOnly>
</template>
