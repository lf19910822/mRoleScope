<template>
  <section class="job-list-skeleton" role="status" aria-live="polite" aria-busy="true">
    <span class="job-list-skeleton__screen-reader">正在读取岗位列表</span>
    <div aria-hidden="true">
      <header class="job-list-skeleton__heading">
        <i class="job-list-skeleton__shape job-list-skeleton__shape--kicker"></i>
        <i class="job-list-skeleton__shape job-list-skeleton__shape--title"></i>
      </header>
      <article v-for="index in 3" :key="index" class="job-list-skeleton__card">
        <div>
          <i class="job-list-skeleton__shape job-list-skeleton__shape--location"></i>
          <i class="job-list-skeleton__shape job-list-skeleton__shape--job-title"></i>
        </div>
        <div class="job-list-skeleton__facts">
          <i v-for="fact in 4" :key="fact" class="job-list-skeleton__shape"></i>
        </div>
        <div class="job-list-skeleton__tags">
          <i v-for="tag in 4" :key="tag" class="job-list-skeleton__shape"></i>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.job-list-skeleton {
  background: #fff;
  border-radius: 16px;
  box-shadow:
    0 5px 12px 4px rgba(11, 37, 50, 0.04),
    0 20px 42px -32px rgba(11, 55, 76, 0.32);
  min-height: 660px;
  padding: 28px;
}
.job-list-skeleton__screen-reader {
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  height: 1px;
  overflow: hidden;
  position: absolute;
  white-space: nowrap;
  width: 1px;
}
.job-list-skeleton__heading {
  display: grid;
  gap: 9px;
  margin-bottom: 20px;
}
.job-list-skeleton__shape {
  animation: job-list-skeleton-shimmer 1.8s ease-in-out infinite;
  background: linear-gradient(105deg, #eaf7fa 20%, #f8fcfd 45%, #eaf7fa 70%);
  background-size: 220% 100%;
  border-radius: 8px;
  display: block;
  height: 14px;
}
.job-list-skeleton__shape--kicker {
  height: 10px;
  width: 76px;
}
.job-list-skeleton__shape--title {
  height: 24px;
  width: 160px;
}
.job-list-skeleton__card {
  border-top: 1px solid #e3ebf0;
  display: grid;
  gap: 18px;
  padding: 22px 0;
}
.job-list-skeleton__card > div:first-child {
  display: grid;
  gap: 10px;
}
.job-list-skeleton__shape--location {
  height: 12px;
  width: 54px;
}
.job-list-skeleton__shape--job-title {
  height: 25px;
  max-width: 400px;
  width: 55%;
}
.job-list-skeleton__facts {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.job-list-skeleton__facts .job-list-skeleton__shape {
  height: 28px;
}
.job-list-skeleton__tags {
  display: flex;
  gap: 7px;
}
.job-list-skeleton__tags .job-list-skeleton__shape {
  height: 22px;
  width: 60px;
}
.job-list-skeleton__tags .job-list-skeleton__shape:nth-child(even) {
  width: 42px;
}

@keyframes job-list-skeleton-shimmer {
  0%,
  100% {
    background-position: 100% 50%;
  }
  50% {
    background-position: 0 50%;
  }
}

@media (max-width: 650px) {
  .job-list-skeleton {
    min-height: 720px;
    padding: 22px;
  }
  .job-list-skeleton__facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    row-gap: 10px;
  }
  .job-list-skeleton__shape--job-title {
    width: 82%;
  }
}

:global(.app-shell[data-color-scheme='dark'] .job-list-skeleton) {
  background: #17303a;
  box-shadow: 0 20px 42px -32px rgba(0, 0, 0, 0.72);
}

:global(.app-shell[data-color-scheme='dark'] .job-list-skeleton__shape) {
  background: linear-gradient(105deg, #1c3944 20%, #294b57 45%, #1c3944 70%);
}

:global(.app-shell[data-color-scheme='dark'] .job-list-skeleton__card) {
  border-top-color: #294752;
}
</style>
