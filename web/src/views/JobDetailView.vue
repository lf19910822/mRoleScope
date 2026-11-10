<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { getJobDetail, type JobDetailResponse } from '@/api/jd-api'
import JDDetailReadingLayout from '@/components/JDDetailReadingLayout.vue'
import JobHistoryTimeline from '@/components/JobHistoryTimeline.vue'
import JobDetailSkeleton from '@/components/JobDetailSkeleton.vue'

const route = useRoute()
const router = useRouter()
const response = ref<JobDetailResponse>()
const error = ref('')
const isLoading = ref(false)
let latestDetailRequest = 0

watch(
  () => String(route.params.id ?? ''),
  async (id) => {
    const request = ++latestDetailRequest
    isLoading.value = true
    error.value = ''
    try {
      const nextResponse = await getJobDetail(id)
      if (request !== latestDetailRequest) return
      response.value = nextResponse
    } catch (reason) {
      if (request !== latestDetailRequest) return
      error.value = reason instanceof Error ? reason.message : '请求岗位详情失败'
    } finally {
      if (request === latestDetailRequest) isLoading.value = false
    }
  },
  { immediate: true },
)

function returnToList(): void {
  router.back()
}
</script>

<template>
  <main class="job-detail-page">
    <button class="back-button" type="button" @click="returnToList">← 返回岗位列表</button>
    <JobDetailSkeleton v-if="isLoading && !response" />
    <p v-else-if="error && !response" class="error-message" role="alert">{{ error }}</p>
    <template v-else-if="response">
      <p v-if="error" class="error-message job-detail-refresh-error" role="alert">{{ error }}</p>
      <JDDetailReadingLayout :job="response" />
      <JobHistoryTimeline :activity-state="response.activity_state" :job-id="response.id" />
    </template>
  </main>
</template>

<style scoped>
.job-detail-page {
  margin: 0 auto;
  max-width: 1040px;
  padding: 52px 32px 88px;
}

.back-button {
  background: transparent;
  border: 0;
  color: #007bab;
  cursor: pointer;
  font: inherit;
  font-weight: 600;
  margin-bottom: 20px;
  padding: 8px 0;
}

.error-message {
  color: #b44953;
}

.job-detail-refresh-error {
  margin: 0 32px 12px;
}

@media (max-width: 700px) {
  .job-detail-page {
    padding: 36px 16px 56px;
  }
}
</style>
