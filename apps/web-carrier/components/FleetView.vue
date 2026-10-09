<script setup lang="ts">
import { Card, CustomerPageHeader, EmptyState, Table } from '@freight-platform/ui'
import { fleetAllowsCreate } from '../domain/fleet'

defineProps<{
  role: string
  drivers: Array<{ id: string; full_name?: string; status?: string }>
  vehicles: Array<{ id: string; plate_number?: string; status?: string }>
}>()
const { t } = useI18n()
</script>

<template>
  <div data-testid="fleet-view">
    <CustomerPageHeader :title="t('carrier.fleetTitle')" />
    <button v-if="fleetAllowsCreate(role)" type="button" data-testid="fleet-create">Create</button>
    <Card>
      <h2>{{ t('carrier.drivers') }}</h2>
      <EmptyState v-if="drivers.length === 0" :title="t('carrier.emptyFleet')" />
      <Table v-else :columns="[t('carrier.drivers'), t('carrier.status')]">
        <tr v-for="driver in drivers" :key="driver.id">
          <td>{{ driver.full_name }}</td>
          <td>{{ driver.status }}</td>
        </tr>
      </Table>
    </Card>
    <Card>
      <h2>{{ t('carrier.vehicles') }}</h2>
      <EmptyState v-if="vehicles.length === 0" :title="t('carrier.emptyFleet')" />
      <Table v-else :columns="[t('carrier.vehicles'), t('carrier.status')]">
        <tr v-for="vehicle in vehicles" :key="vehicle.id">
          <td>{{ vehicle.plate_number }}</td>
          <td>{{ vehicle.status }}</td>
        </tr>
      </Table>
    </Card>
  </div>
</template>
