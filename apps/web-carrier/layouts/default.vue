<script setup lang="ts">
import { Button, CustomerAppShell, CustomerNavigation, LocaleSwitcher } from '@freight-platform/ui'
import CompanyGate from '../components/CompanyGate.vue'

const office = useCarrierOffice()
const route = useRoute()
const { t } = useI18n()
const localePath = useLocalePath()

const items = computed(() => [
  { href: localePath('/'), label: t('common.home'), active: route.path === localePath('/') },
  { href: localePath('/tenders'), label: t('carrier.navTenders'), active: route.path.startsWith(localePath('/tenders')) },
  { href: localePath('/transport-orders'), label: t('carrier.navOrders'), active: route.path.startsWith(localePath('/transport-orders')) },
  { href: localePath('/fleet'), label: t('carrier.navFleet'), active: route.path.startsWith(localePath('/fleet')) },
])

function onSelect(companyId: string) {
  office.chooseCompany(companyId)
}
</script>

<template>
  <ClientOnly>
    <CustomerAppShell :title="t('carrier.product')">
      <template #nav>
        <CustomerNavigation :items="items" />
      </template>
      <template #context>
        <span v-if="office.session.value">{{ office.session.value.user.email }}</span>
      </template>
      <template #actions>
        <LocaleSwitcher />
        <Button variant="ghost" type="button" data-testid="sign-out" @click="office.signOut()">
          {{ t('carrier.signOut') }}
        </Button>
      </template>
      <CompanyGate
        v-if="!office.selectedCompanyId.value"
        :companies="office.companies.value"
        @select="onSelect"
      />
      <slot v-else />
    </CustomerAppShell>
    <template #fallback>
      <p>{{ t('carrier.loading') }}</p>
    </template>
  </ClientOnly>
</template>
