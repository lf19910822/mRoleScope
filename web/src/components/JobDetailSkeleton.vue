<template>
  <section class="job-detail-skeleton" role="status" aria-live="polite" aria-busy="true">
    <span class="job-detail-skeleton__screen-reader">正在读取岗位详情</span>
    <article class="job-detail-skeleton__sheet" aria-hidden="true">
      <header class="job-detail-skeleton__header">
        <i class="job-detail-skeleton__shape job-detail-skeleton__shape--kicker"></i>
        <i class="job-detail-skeleton__shape job-detail-skeleton__shape--title"></i>
        <i class="job-detail-skeleton__shape job-detail-skeleton__shape--title-short"></i>
        <i class="job-detail-skeleton__shape job-detail-skeleton__shape--location"></i>
      </header>
      <div class="job-detail-skeleton__facts">
        <i v-for="index in 4" :key="index" class="job-detail-skeleton__shape"></i>
      </div>
      <div class="job-detail-skeleton__technologies">
        <i class="job-detail-skeleton__shape job-detail-skeleton__shape--technology-label"></i>
        <i v-for="index in 7" :key="index" class="job-detail-skeleton__shape"></i>
      </div>
      <div class="job-detail-skeleton__sections">
        <section v-for="index in 3" :key="index" class="job-detail-skeleton__section">
          <i class="job-detail-skeleton__shape job-detail-skeleton__shape--section-label"></i>
          <div v-for="item in 2" :key="item" class="job-detail-skeleton__item">
            <i class="job-detail-skeleton__ring"></i>
            <span>
              <i class="job-detail-skeleton__shape"></i>
              <i class="job-detail-skeleton__shape job-detail-skeleton__shape--copy-short"></i>
            </span>
          </div>
        </section>
      </div>
    </article>
  </section>
</template>

<style scoped>
.job-detail-skeleton {
  min-height: 670px;
}
.job-detail-skeleton__screen-reader {
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  height: 1px;
  overflow: hidden;
  position: absolute;
  white-space: nowrap;
  width: 1px;
}
.job-detail-skeleton__sheet {
  background: #fff;
  border-radius: 26px;
  box-shadow: 0 22px 50px -34px rgba(11, 55, 76, 0.42);
  overflow: hidden;
}
.job-detail-skeleton__header {
  background: linear-gradient(126deg, #edf9fc 0%, #fff 72%);
  display: grid;
  gap: 12px;
  padding: 36px 42px 31px;
}
.job-detail-skeleton__shape,
.job-detail-skeleton__ring {
  background: linear-gradient(105deg, #eaf7fa 20%, #f8fcfd 45%, #eaf7fa 70%);
  background-size: 220% 100%;
  border-radius: 8px;
  display: block;
  height: 13px;
  animation: job-detail-skeleton-shimmer 1.8s ease-in-out infinite;
}
.job-detail-skeleton__shape--kicker {
  height: 9px;
  width: 104px;
}
.job-detail-skeleton__shape--title {
  height: 42px;
  max-width: 580px;
  width: 72%;
}
.job-detail-skeleton__shape--title-short {
  height: 42px;
  max-width: 360px;
  width: 48%;
}
.job-detail-skeleton__shape--location {
  height: 14px;
  width: 52px;
}
.job-detail-skeleton__facts {
  display: grid;
  gap: 18px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  padding: 22px 42px;
}
.job-detail-skeleton__facts .job-detail-skeleton__shape {
  height: 30px;
}
.job-detail-skeleton__technologies {
  align-items: center;
  border-bottom: 1px solid #e3ebf0;
  border-top: 1px solid #e3ebf0;
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  padding: 16px 42px;
}
.job-detail-skeleton__technologies .job-detail-skeleton__shape {
  height: 22px;
  width: 64px;
}
.job-detail-skeleton__technologies .job-detail-skeleton__shape:nth-child(3n) {
  width: 48px;
}
.job-detail-skeleton__technologies .job-detail-skeleton__shape--technology-label {
  height: 12px;
  width: 58px;
}
.job-detail-skeleton__sections {
  padding: 6px 42px 42px;
}
.job-detail-skeleton__section {
  border-top: 1px solid #e6edf1;
  padding: 29px 0 0;
}
.job-detail-skeleton__section:first-child {
  border-top: 0;
}
.job-detail-skeleton__section + .job-detail-skeleton__section {
  margin-top: 30px;
}
.job-detail-skeleton__shape--section-label {
  height: 18px;
  margin-bottom: 14px;
  width: 82px;
}
.job-detail-skeleton__item {
  align-items: center;
  background: #f3fafc;
  border: 1px solid #dceef2;
  border-radius: 15px;
  display: grid;
  gap: 12px;
  grid-template-columns: 30px minmax(0, 1fr);
  padding: 15px;
}
.job-detail-skeleton__item + .job-detail-skeleton__item {
  margin-top: 11px;
}
.job-detail-skeleton__ring {
  border: 1.5px solid #b9e2ea;
  border-radius: 50%;
  box-sizing: border-box;
  height: 30px;
  width: 30px;
}
.job-detail-skeleton__item span {
  display: grid;
  gap: 8px;
}
.job-detail-skeleton__shape--copy-short {
  width: 68%;
}

@keyframes job-detail-skeleton-shimmer {
  0%,
  100% {
    background-position: 100% 50%;
  }
  50% {
    background-position: 0 50%;
  }
}

@media (max-width: 650px) {
  .job-detail-skeleton {
    min-height: 720px;
  }
  .job-detail-skeleton__header,
  .job-detail-skeleton__facts,
  .job-detail-skeleton__technologies,
  .job-detail-skeleton__sections {
    padding-left: 22px;
    padding-right: 22px;
  }
  .job-detail-skeleton__facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .job-detail-skeleton__shape--title {
    height: 34px;
    width: 90%;
  }
  .job-detail-skeleton__shape--title-short {
    height: 34px;
    width: 65%;
  }
}
</style>
