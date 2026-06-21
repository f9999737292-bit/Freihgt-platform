<script setup lang="ts">
import { formatDocumentDate, type DocumentFile } from '~/types/document'

defineProps<{ files: DocumentFile[]; loading?: boolean }>()
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('documents.files') }}</h3>
    </template>

    <div v-if="loading" class="text-muted">{{ $t('common.loading') }}</div>
    <UiEmptyState v-else-if="!files.length" :title="$t('documents.noFiles')" />

    <UiTable
      v-else
      :columns="[
        $t('documents.fileType'),
        $t('documents.fileName'),
        $t('documents.mimeType'),
        $t('documents.fileSize'),
        $t('documents.storageProvider'),
        $t('documents.bucketName'),
        $t('documents.objectKey'),
        $t('documents.checksum'),
      ]"
    >
      <tr v-for="file in files" :key="file.id">
        <td>{{ file.file_type }}</td>
        <td>{{ file.file_name || '—' }}</td>
        <td>{{ file.mime_type || '—' }}</td>
        <td>{{ file.file_size_bytes ?? '—' }}</td>
        <td>{{ file.storage_provider }}</td>
        <td>{{ file.bucket_name || '—' }}</td>
        <td>{{ file.object_key }}</td>
        <td>{{ file.checksum_sha256 || '—' }}</td>
      </tr>
    </UiTable>
  </UiCard>
</template>

<style scoped>
.card-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}
</style>
